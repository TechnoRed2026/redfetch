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
	user, host, os, arch, cpu, gpu, ram, shell, ip, uptime string
	disks                                                  []string
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
	par(func() { in.gpu = gpuName(ctx) })
	par(func() { in.shell = shellName(ctx) })
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
		`       ▄▄▄▄▄▄▄       `,
		`    ▄███████████▄    `,
		`  ▄███▀       ▀███▄  `,
		` ▐███   ▄███▄   ███▌ `,
		` ▐███   ▀███▀   ███▌ `,
		`  ▀███▄       ▄███▀  `,
		`    ▀███████████▀    `,
		`       ▀▀▀▀▀▀▀       `,
	}
	type kv struct{ k, v string }
	items := []kv{
		{"OS", in.os},
		{"ARCH", in.arch},
		{"CPU", in.cpu},
		{"GPU", in.gpu},
		{"RAM", in.ram},
		{"SHELL", in.shell},
	}
	for i, d := range in.disks {
		label := "DISK"
		if len(in.disks) > 1 {
			label = fmt.Sprintf("DISK%d", i+1)
		}
		items = append(items, kv{label, d})
	}
	items = append(items, kv{"IP", in.ip}, kv{"UP", in.uptime})

	// inner width from visible text: "KEY:" padded to keyW, then value
	keyW := 0
	for _, it := range items {
		if n := len([]rune(it.k)) + 1; n > keyW {
			keyW = n
		}
	}
	keyW++ // gap after colon
	title := in.user + "@" + in.host
	titleW := len([]rune(title)) + 3 // "─ " + title + " "
	inner := titleW
	const barW = 24 // 8 swatches × 3 blocks
	if inner < barW+2 {
		inner = barW + 2
	}
	for _, it := range items {
		if n := keyW + len([]rune(fallback(it.v, "-"))); n > inner {
			inner = n
		}
	}
	inner += 2 // padding inside borders

	rows := []string{p.label + "╭─ " + p.reset + p.red + title + p.reset + p.label + " " + strings.Repeat("─", inner-titleW) + "╮" + p.reset}
	for _, it := range items {
		v := fallback(it.v, "-")
		pad := strings.Repeat(" ", inner-keyW-len([]rune(v))-1)
		rows = append(rows, fmt.Sprintf("%s│ %s%s:%s%s%s%s%s%s%s│%s",
			p.label, p.red2, it.k, p.reset,
			strings.Repeat(" ", keyW-len([]rune(it.k))-1),
			p.text, v, p.reset, pad, p.label, p.reset))
	}
	if color {
		blank := p.label + "│" + strings.Repeat(" ", inner) + "│" + p.reset
		rows = append(rows, blank,
			p.label+"│ "+p.reset+bar()+strings.Repeat(" ", inner-barW-1)+p.label+"│"+p.reset)
	}
	rows = append(rows, p.label+"╰"+strings.Repeat("─", inner)+"╯"+p.reset)
	var b strings.Builder
	for i := 0; i < len(logo) || i < len(rows); i++ {
		b.WriteString("  ")
		if i < len(logo) {
			fmt.Fprintf(&b, "%s%s%s", logoColor(p, i), logo[i], p.reset)
		} else {
			b.WriteString(strings.Repeat(" ", len([]rune(logo[0]))))
		}
		if i < len(rows) {
			fmt.Fprintf(&b, "   %s", rows[i])
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

// bar renders the neofetch-signature terminal color swatches (ANSI 0-7).
func bar() string {
	var b strings.Builder
	for c := 0; c < 8; c++ {
		fmt.Fprintf(&b, "\x1b[3%dm███", c)
	}
	b.WriteString("\x1b[0m")
	return b.String()
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
		// "Windows 10 Pro" from registry, like winfetch
		name := regSZ(run(ctx, "reg", "query", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "/v", "ProductName"))
		if name == "" {
			return fallback(run(ctx, "cmd", "/c", "ver"), "Windows")
		}
		if build := regSZ(run(ctx, "reg", "query", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "/v", "CurrentBuildNumber")); build != "" {
			name += " (build " + build + ")"
		}
		return name
	}
	// distro name from os-release, fallback to kernel
	if s := run(ctx, "sh", "-c", `. /etc/os-release 2>/dev/null && echo "$PRETTY_NAME"`); s != "" {
		return s
	}
	return fallback(run(ctx, "uname", "-sr"), runtime.GOOS)
}

func cpuName(ctx context.Context) string {
	if runtime.GOOS == "windows" {
		out := run(ctx, "reg", "query", `HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "/v", "ProcessorNameString")
		if fields := strings.Fields(out); len(fields) > 3 {
			return cleanCPU(strings.Join(fields[3:], " "))
		}
		return runtime.GOARCH
	}
	if s := run(ctx, "sh", "-c", "lscpu 2>/dev/null | sed -n 's/^Model name:[[:space:]]*//p' | head -n1"); s != "" {
		return cleanCPU(s)
	}
	return runtime.GOARCH
}

// regSZ extracts the value from `reg query` REG_SZ output.
func regSZ(out string) string {
	if i := strings.Index(out, "REG_SZ"); i >= 0 {
		return strings.TrimSpace(out[i+6:])
	}
	return ""
}

func gpuName(ctx context.Context) string {
	if runtime.GOOS == "windows" {
		out := run(ctx, "powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_VideoController | Select-Object -First 1).Name")
		return fallback(out, "-")
	}
	if s := run(ctx, "sh", "-c", `lspci 2>/dev/null | grep -iE 'vga|3d|display' | head -n1 | sed 's/^.*: //'`); s != "" {
		return s
	}
	return "-"
}

// shellName guesses the invoking shell from env — cheap and good enough.
// ponytail: parent-process walk would be exact; add if env guess proves wrong.
func shellName(ctx context.Context) string {
	if s := os.Getenv("SHELL"); s != "" { // unix / git-bash
		parts := strings.Split(strings.ReplaceAll(s, `\`, "/"), "/")
		return strings.TrimSuffix(parts[len(parts)-1], ".exe")
	}
	if os.Getenv("PSModulePath") != "" {
		return "powershell"
	}
	if os.Getenv("PROMPT") != "" {
		return "cmd"
	}
	return "-"
}

// cleanCPU strips marketing noise: "Intel(R) Core(TM) i5-4590 CPU @ 3.30GHz" -> "Intel Core i5-4590 @ 3.30GHz"
func cleanCPU(s string) string {
	for _, junk := range []string{"(R)", "(TM)", "(tm)", " CPU", " Processor"} {
		s = strings.ReplaceAll(s, junk, "")
	}
	return strings.Join(strings.Fields(s), " ")
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
	// real partitions only: /dev/* or drive-letter (WSL drvfs), no loop devices, no snap/WSL internals
	out := run(ctx, "sh", "-c", `df -h -x tmpfs -x devtmpfs 2>/dev/null | awk 'NR>1 && ($1 ~ "^/dev/" || $1 ~ "^[A-Za-z]:") && $1 !~ "loop" && $6 !~ "^/(snap|boot|init|usr/lib/wsl|mnt/wslg)" {print $6 ": " $3 " / " $2}'`)
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
