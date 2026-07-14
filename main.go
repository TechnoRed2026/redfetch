package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type info struct{ user, host, os, arch, cpu, ram, disk, ip, git, uptime string }

func main() { fmt.Print(render(collect(context.Background()), true)) }

func collect(ctx context.Context) info {
	host, _ := os.Hostname()
	return info{
		user:   getenv("USERNAME", getenv("USER", "user")),
		host:   fallback(host, "host"),
		os:     osName(ctx),
		arch:   runtime.GOARCH,
		cpu:    cpuName(ctx),
		ram:    ram(ctx),
		disk:   disk(ctx),
		ip:     firstIP(),
		git:    gitBranch(ctx),
		uptime: uptime(ctx),
	}
}

func render(in info, color bool) string {
	red, dim, reset := "", "", ""
	if color {
		red, dim, reset = "\x1b[31;1m", "\x1b[2m", "\x1b[0m"
	}
	logo := []string{"██████╗ ", "██╔══██╗", "██████╔╝", "██╔══██╗", "██║  ██║", "╚═╝  ╚═╝"}
	rows := []string{
		fmt.Sprintf("%s%s@%s%s", red, in.user, in.host, reset),
		kv("os", in.os), kv("arch", in.arch), kv("cpu", in.cpu), kv("ram", in.ram), kv("disk", in.disk), kv("ip", in.ip), kv("git", in.git), kv("up", in.uptime),
	}
	var b strings.Builder
	for i, art := range logo {
		line := ""
		if i < len(rows) {
			line = rows[i]
		}
		fmt.Fprintf(&b, "%s%s%s  %s%s%s\n", red, art, reset, dim, line, reset)
	}
	for _, line := range rows[len(logo):] {
		fmt.Fprintf(&b, "          %s%s%s\n", dim, line, reset)
	}
	return b.String()
}

func kv(k, v string) string { return fmt.Sprintf("%-6s %s", k+":", fallback(v, "-")) }

func run(ctx context.Context, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(ctx, 700*time.Millisecond)
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
	if runtime.GOOS == "windows" {
		return fallback(run(ctx, "powershell", "-NoProfile", "-Command", "$m=Get-CimInstance Win32_OperatingSystem; '{0:N1} GB / {1:N1} GB' -f (($m.TotalVisibleMemorySize-$m.FreePhysicalMemory)/1MB),($m.TotalVisibleMemorySize/1MB)"), "-")
	}
	if s := run(ctx, "sh", "-c", "free -h 2>/dev/null | awk '/Mem:/ {print $3 \" / \" $2}'"); s != "" {
		return s
	}
	return "-"
}

func disk(ctx context.Context) string {
	if runtime.GOOS == "windows" {
		return fallback(run(ctx, "powershell", "-NoProfile", "-Command", "$d=Get-CimInstance Win32_LogicalDisk -Filter \"DeviceID='C:'\"; '{0:N1} GB / {1:N1} GB' -f (($d.Size-$d.FreeSpace)/1GB),($d.Size/1GB)"), "-")
	}
	if s := run(ctx, "sh", "-c", "df -h / 2>/dev/null | awk 'NR==2 {print $3 \" / \" $2}'"); s != "" {
		return s
	}
	return "-"
}

func uptime(ctx context.Context) string {
	if runtime.GOOS == "windows" {
		return fallback(run(ctx, "powershell", "-NoProfile", "-Command", "$u=(Get-Date)-(Get-CimInstance Win32_OperatingSystem).LastBootUpTime; '{0}d {1}h {2}m' -f $u.Days,$u.Hours,$u.Minutes"), "-")
	}
	return fallback(run(ctx, "sh", "-c", "uptime -p 2>/dev/null | sed 's/^up //'"), "-")
}

func gitBranch(ctx context.Context) string {
	return fallback(run(ctx, "git", "branch", "--show-current"), "-")
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
