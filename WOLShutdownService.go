package main

import (
    "bytes"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "golang.org/x/sys/windows/svc"
    "golang.org/x/sys/windows/svc/eventlog"
)

const serviceName = "WOLShutdownService"

type Config struct {
    MAC        string   `json:"mac"`
    AllowedIPs []string `json:"allowed_ips"`
}

func normalizeMAC(mac string) ([]byte, error) {
    clean := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(mac), ":", ""), "-", "")
    if len(clean) != 12 {
        return nil, fmt.Errorf("MAC must be 12 hex chars after stripping separators, got %q", clean)
    }
    return hex.DecodeString(clean)
}

func isMagicPacket(data []byte, mac []byte) bool {
    if len(data) < 6+16*len(mac) {
        return false
    }
    for i := 0; i < 6; i++ {
        if data[i] != 0xFF {
            return false
        }
    }
    expected := bytes.Repeat(mac, 16)
    return bytes.Equal(data[6:6+len(expected)], expected)
}

func loadConfig(cfgPath string) (*Config, error) {
    data, err := os.ReadFile(cfgPath)
    if err != nil {
        return nil, err
    }
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

type wolService struct {
    el       *eventlog.Log
    macBytes []byte
    allowed  map[string]bool
}

func (ws *wolService) Execute(args []string, changes <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
    const accepts = svc.AcceptStop | svc.AcceptShutdown
    status <- svc.Status{State: svc.StartPending}
    status <- svc.Status{State: svc.Running, Accepts: accepts}
    ws.el.Info(1, fmt.Sprintf("%s started", serviceName))

    addr := net.UDPAddr{Port: 9, IP: net.IPv4zero}
    conn, err := net.ListenUDP("udp", &addr)
    if err != nil {
        ws.el.Error(1, fmt.Sprintf("Failed to bind UDP port 9: %v", err))
        status <- svc.Status{State: svc.StopPending}
        return false, 1
    }
    defer conn.Close()

    ws.el.Info(1, "Listening for WOL packets on UDP port 9")
    buf := make([]byte, 2048)

loop:
    for {
        select {
        case c := <-changes:
            switch c.Cmd {
            case svc.Interrogate:
                status <- c.CurrentStatus
            case svc.Stop, svc.Shutdown:
                ws.el.Info(1, "Stop requested, exiting service loop")
                break loop
            }
        default:
            _ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
            n, remote, err := conn.ReadFromUDP(buf)
            if err != nil {
                continue
            }

            if len(ws.allowed) > 0 && !ws.allowed[remote.IP.String()] {
                ws.el.Info(1, fmt.Sprintf("Packet ignored from %s (not allowlisted)", remote.IP.String()))
                continue
            }

            if isMagicPacket(buf[:n], ws.macBytes) {
                ws.el.Info(1, fmt.Sprintf("Valid WOL packet from %s — initiating shutdown", remote.IP.String()))
                if err := exec.Command("shutdown", "/s", "/t", "0").Run(); err != nil {
                    ws.el.Error(1, fmt.Sprintf("Shutdown command failed: %v", err))
                } else {
                    ws.el.Info(1, "Shutdown command executed")
                }
            } else {
                ws.el.Info(1, fmt.Sprintf("Non-matching packet ignored from %s", remote.IP.String()))
            }
        }
    }

    status <- svc.Status{State: svc.StopPending}
    return false, 0
}

func main() {
    // Ensure Event Log source exists
    el, err := eventlog.Open(serviceName)
    if err != nil {
        _ = eventlog.InstallAsEventCreate(serviceName, eventlog.Info|eventlog.Error)
        el, err = eventlog.Open(serviceName)
        if err != nil {
            panic(err)
        }
    }
    defer el.Close()

    // Resolve config.ini path relative to executable
    exePath, err := os.Executable()
    if err != nil {
        el.Error(1, fmt.Sprintf("Cannot determine executable path: %v", err))
        return
    }
    cfgPath := filepath.Join(filepath.Dir(exePath), "config.ini")

    cfg, err := loadConfig(cfgPath)
    if err != nil {
        el.Error(1, fmt.Sprintf("Failed to load config file %s: %v", cfgPath, err))
        return
    }
    macBytes, err := normalizeMAC(cfg.MAC)
    if err != nil {
        el.Error(1, fmt.Sprintf("Invalid MAC in config: %v", err))
        return
    }
    allowed := make(map[string]bool, len(cfg.AllowedIPs))
    for _, ip := range cfg.AllowedIPs {
        allowed[strings.TrimSpace(ip)] = true
    }

    isSvc, err := svc.IsWindowsService()
    if err != nil {
        el.Error(1, fmt.Sprintf("Service detection failed: %v", err))
        return
    }

    if isSvc {
        if err := svc.Run(serviceName, &wolService{
            el:       el,
            macBytes: macBytes,
            allowed:  allowed,
        }); err != nil {
            el.Error(1, fmt.Sprintf("svc.Run failed: %v", err))
        }
    } else {
        // Console mode for testing
        el.Info(1, "Running in console mode (not under SCM)")
        ws := &wolService{el: el, macBytes: macBytes, allowed: allowed}
        stop := make(chan svc.ChangeRequest)
        st := make(chan svc.Status)
        go func() {
            time.Sleep(30 * time.Second)
            stop <- svc.ChangeRequest{Cmd: svc.Stop}
        }()
        ws.Execute(nil, stop, st)
    }
}
