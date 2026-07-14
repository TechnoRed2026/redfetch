package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const version = "0.2.0"

type info struct {
	user, host, os, arch, cpu, ram, ip, uptime string
	disks                                      []string
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("redfetch " + version)
		return
	}
	fmt.Print(render(collect(context.Background()), useColor()))
}

// color off when piped or NO_COLOR set; on Windows also requires VT support.
func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	st, err := os.Stdout.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return enableVT()
}

func collect(ctx context.Context) info {
	host, _ := os.Hostname()
	in := info{
		user: getenv("USERNAME", getenv("USER", "user")),
		host: fallback(host, "host"),
		arch: runtime.GOARCH,
	}
	var wg sync.WaitGroup
	par := func(f func()) { wg.Add(1); go func() { defer wg.Done(); f() }() }
	par(func() { in.os = osName(ctx) })
	par(func() { in.cpu = cpuName(ctx) })
	par(func() { in.ram = ram(ctx) })
	par(func() { in.disks = disks(ctx) })
	par(func() { in.ip = firstIP() })
	par(func() { in.uptime = uptime(ctx) })
	wg.Wait()
	return in
}

type palette struct{ red, red2, pink, orange, label, text, dim, reset string }

func render(in info, color bool) string {
	p := colors(color)
	logo := []string{
		"        ▄▄▄▄▄▄▄        ",
		"     ▄███████████▄     ",
		"   ▄███▀       ▀███▄   ",
		"  ▐███   ▄███▄   ███▌  ",
		"  ▐███   ▀███▀   ███▌  ",
		"   ▀███▄       ▄███▀   ",
		"     ▀███████████▀     ",
		"        ▀▀▀▀▀▀▀        ",
	}
	rows := []string{
		fmt.Sprintf("%s╭─ redfetch%s", p.red, p.reset),
		line(p, "ᴜsᴇʀ", in.user+"@"+in.host),
		line(p, "ᴏs", in.os),
		line(p, "ᴀʀᴄʜ", in.arch),
		line(p, "ᴄᴘᴜ", in.cpu),
		line(p, "ʀᴀᴍ", in.ram),
	}
	for i, d := range in.disks {
		label := "ᴅɪsᴋ"
		if len(in.disks) > 1 {
			label = fmt.Sprintf("ᴅɪsᴋ%d", i+1)
		}
		rows = append(rows, line(p, label, d))
	}
	rows = append(rows,
		line(p, "ɪᴘ", in.ip),
		line(p, "ᴜᴘ", in.uptime),
		fmt.Sprintf("%s╰──────────%s", p.red, p.reset),
	)
	var b strings.Builder
	for i := 0; i < len(logo) || i < len(rows); i++ {
		if i < len(logo) {
			fmt.Fprintf(&b, "%s%s%s", logoColor(p, i), logo[i], p.reset)
		} else {
			b.WriteString(strings.Repeat(" ", len([]rune(logo[0]))))
		}
		if i < len(rows) {
			fmt.Fprintf(&b, "  %s", rows[i])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func colors(on bool) palette {
	if !on {
		return palette{}
	}
	return palette{
		red:    "\x1b[38;5;196;1m",
		red2:   "\x1b[38;5;203m",
		pink:   "\x1b[38;5;197m",
		orange: "\x1b[38;5;208m",
		label:  "\x1b[38;5;245m",
		text:   "\x1b[38;5;231m",
		dim:    "\x1b[2m",
		reset:  "\x1b[0m",
	}
}

func logoColor(p palette, i int) string {
	return []string{p.red, p.red, p.red2, p.red2, p.pink, p.pink, p.orange, p.orange}[i%8]
}

func line(p palette, k, v string) string {
	return fmt.Sprintf("%s│%s %s%-8s%s %s%s%s", p.red2, p.reset, p.label, k+":", p.reset, p.text, fallback(v, "-"), p.reset)
}

func run(ctx context.Context, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func osName(ctx context.Context) string {
	if runtime.GOOS == "windows" {
		return fallback(run(ctx, "cmd", "/c", "ver"), "Windows")
	}
	return fallback(run(ctx, "uname", "-sr"), runtime.GOOS)
}

func cpuName(ctx context.Context) string {
	if runtime.GOOS == "windows" {
		out := run(ctx, "reg", "query", `HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "/v", "ProcessorNameString")
		if fields := strings.Fields(out); len(fields) > 3 {
			return strings.Join(fields[3:], " ")
		}
		return runtime.GOARCH
	}
	if s := run(ctx, "sh", "-c", "lscpu 2>/dev/null | sed -n 's/^Model name:[[:space:]]*//p' | head -n1"); s != "" {
		return s
	}
	return runtime.GOARCH
}

func ram(ctx context.Context) string {
	if s := nativeRAM(); s != "" {
		return s
	}
	if runtime.GOOS == "windows" {
		return fallback(run(ctx, "powershell", "-NoProfile", "-Command", "$m=Get-CimInstance Win32_OperatingSystem; '{0:N1} GB / {1:N1} GB' -f (($m.TotalVisibleMemorySize-$m.FreePhysicalMemory)/1MB),($m.TotalVisibleMemorySize/1MB)"), "-")
	}
	if s := run(ctx, "sh", "-c", "free -h 2>/dev/null | awk '/Mem:/ {print $3 \" / \" $2}'"); s != "" {
		return s
	}
	return "-"
}

func disks(ctx context.Context) []string {
	if d := nativeDisks(); len(d) > 0 {
		return d
	}
	if runtime.GOOS == "windows" {
		out := run(ctx, "powershell", "-NoProfile", "-Command", "Get-PSDrive -PSProvider FileSystem | Where-Object {$_.Used -ne $null -and ($_.Used+$_.Free) -gt 0} | ForEach-Object { '{0}: {1:N1} GB / {2:N1} GB' -f $_.Name,($_.Used/1GB),(($_.Used+$_.Free)/1GB) }")
		return lines(out)
	}
	out := run(ctx, "sh", "-c", "df -h -x tmpfs -x devtmpfs 2>/dev/null | awk 'NR>1 {print $6 \": \" $3 \" / \" $2}'")
	return lines(out)
}

func uptime(ctx context.Context) string {
	if s := nativeUptime(); s != "" {
		return s
	}
	if runtime.GOOS == "windows" {
		return fallback(run(ctx, "powershell", "-NoProfile", "-Command", "$u=(Get-Date)-(Get-CimInstance Win32_OperatingSystem).LastBootUpTime; '{0}d {1}h {2}m' -f $u.Days,$u.Hours,$u.Minutes"), "-")
	}
	return fallback(run(ctx, "sh", "-c", "uptime -p 2>/dev/null | sed 's/^up //'"), "-")
}

func lines(s string) []string {
	out := strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' })
	if len(out) == 0 {
		return []string{"-"}
	}
	return out
}

func firstIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 200*time.Millisecond)
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP.To4() != nil {
			return addr.IP.String()
		}
	}
	ifs, err := net.Interfaces()
	if err != nil {
		return "-"
	}
	for _, iface := range ifs {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip, _, _ := net.ParseCIDR(addr.String())
			if ip4 := ip.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return "-"
}

func getenv(k, fb string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fb
}
func fallback(v, fb string) string {
	if strings.TrimSpace(v) == "" {
		return fb
	}
	return strings.TrimSpace(v)
}
