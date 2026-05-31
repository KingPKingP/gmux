package tmux

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gmux/internal/config"
)

func Install(cfg config.Config) error {
	if _, err := exec.LookPath(cfg.TmuxBin); err == nil {
		fmt.Printf("tmux already installed: %s\n", cfg.TmuxBin)
	} else {
		cmd, err := installCommand()
		if err != nil {
			return err
		}

		fmt.Printf("running installer: %s\n", strings.Join(cmd, " "))
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("install failed: %w", err)
		}

		if _, err := exec.LookPath(cfg.TmuxBin); err != nil {
			return fmt.Errorf("tmux still not found after install; restart shell or set tmux_bin in config")
		}
		fmt.Println("tmux install complete")
	}

	if err := installSelfBinary(); err != nil {
		return err
	}
	return nil
}

func installCommand() ([]string, error) {
	asRoot := os.Geteuid() == 0
	withPriv := func(args ...string) []string {
		if asRoot {
			return args
		}
		if has("sudo") {
			return append([]string{"sudo"}, args...)
		}
		return args
	}

	switch runtime.GOOS {
	case "darwin":
		if has("brew") {
			return []string{"brew", "install", "tmux"}, nil
		}
	case "linux":
		switch {
		case has("apt-get"):
			if asRoot || has("sudo") {
				prefix := ""
				if !asRoot {
					prefix = "sudo "
				}
				return []string{"sh", "-c", prefix + "apt-get update && " + prefix + "apt-get install -y tmux"}, nil
			}
			return []string{"sh", "-c", "apt-get update && apt-get install -y tmux"}, nil
		case has("dnf"):
			return withPriv("dnf", "install", "-y", "tmux"), nil
		case has("yum"):
			return withPriv("yum", "install", "-y", "tmux"), nil
		case has("pacman"):
			return withPriv("pacman", "-S", "--noconfirm", "tmux"), nil
		case has("apk"):
			return withPriv("apk", "add", "tmux"), nil
		}
	case "windows":
		if has("winget") {
			return []string{"winget", "install", "--id", "GnuWin32.Tmux", "-e"}, nil
		}
		if has("choco") {
			return []string{"choco", "install", "tmux", "-y"}, nil
		}
	}
	return nil, fmt.Errorf("no supported package manager detected; install tmux manually")
}

func has(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func installSelfBinary() error {
	if runtime.GOOS == "windows" {
		fmt.Println("gmux self-install to /usr/bin style path is not supported on windows")
		return nil
	}

	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve current gmux binary: %w", err)
	}
	if resolved, rerr := filepath.EvalSymlinks(src); rerr == nil {
		src = resolved
	}

	targets := []string{"/usr/local/bin/gmux", "/usr/bin/gmux"}
	for _, dst := range targets {
		if samePath(src, dst) {
			fmt.Printf("gmux already installed: %s\n", dst)
			return nil
		}
		if err := copyFile(src, dst); err == nil {
			fmt.Printf("gmux installed: %s\n", dst)
			return nil
		}
	}

	// Retry with sudo where available.
	if has("sudo") {
		for _, dst := range targets {
				if samePath(src, dst) {
					fmt.Printf("gmux already installed: %s\n", dst)
					return nil
				}
			cmd := exec.Command("sudo", "cp", src, dst)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				chmodCmd := exec.Command("sudo", "chmod", "755", dst)
				chmodCmd.Stdin = os.Stdin
				chmodCmd.Stdout = os.Stdout
				chmodCmd.Stderr = os.Stderr
				_ = chmodCmd.Run()
					fmt.Printf("gmux installed: %s\n", dst)
				return nil
			}
		}
	}

	return fmt.Errorf("failed to install gmux into /usr/local/bin or /usr/bin (permission denied)")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func samePath(a, b string) bool {
	aa := a
	bb := b
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		aa = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		bb = rb
	}
	return aa == bb
}
