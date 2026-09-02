package cmd

import (
	"fmt"
	"strings"

	"github.com/andreibanu/pusher/internal/robot"
	"github.com/spf13/cobra"
)

var ipAdapters bool

var ipCmd = &cobra.Command{
	Use:   "ip",
	Short: "Show where the robot is and what it is serving",
	Long: `Prints the robot's addresses and every port worth knowing about: the hub's
own manage page, FtcDashboard, Panels, and the Limelight through the Panels
proxy.

Only what actually answered is reported, so an address printed here is one you
can paste into a browser rather than one pusher thinks ought to work.`,
	RunE: runIP,
}

func init() {
	ipCmd.Flags().BoolVar(&ipAdapters, "adapters", false,
		"Report the robot's network interfaces and what is on its USB bus")
}

func runIP(cmd *cobra.Command, args []string) error {
	serial, err := requireRobot()
	if err != nil {
		return err
	}

	if ipAdapters {
		return reportAdapters(serial)
	}

	survey, err := robot.Take(serial)
	if err != nil {
		return err
	}

	fmt.Printf("\n%s\n", strings.TrimSpace(survey.Model+" ("+survey.Serial+")"))
	fmt.Println("─────────────────────────────────────────")

	if len(survey.Addresses) > 0 {
		fmt.Printf("  addresses    %s\n", strings.Join(survey.Addresses, ", "))
	} else {
		fmt.Println("  addresses    none the robot would report")
	}

	if survey.Hostname != "" {
		fmt.Printf("  hostname     %s\n", survey.Hostname)
	}
	if survey.Local != "" {
		fmt.Printf("  .local       %s\n", survey.Local)
	} else if survey.Hostname != "" {
		fmt.Printf("  .local       %s.local does not resolve from here\n", survey.Hostname)
	}

	if survey.Host == "" {
		fmt.Println("\n  Nothing to knock on: pusher reached this robot over USB and it")
		fmt.Println("  reported no address, so it is not on a network you can open.")
		return nil
	}

	fmt.Println()
	for _, service := range survey.Services {
		mark := "  -"
		if service.Reachable {
			mark = "  ✓"
		}

		where := service.URL(survey.Host)
		if where == "" {
			where = fmt.Sprintf("%s:%d", survey.Host, service.Port)
		}
		if !service.Reachable {
			where = "not answering on " + fmt.Sprintf("%d", service.Port)
		}

		fmt.Printf("%s %-18s %-34s %s\n", mark, service.Name, where, service.Note)
	}

	if survey.Local != "" {
		fmt.Printf("\n  %s works anywhere %s does, and survives the robot changing address.\n",
			survey.Local, survey.Host)
	}

	return nil
}

// reportAdapters says what the hub has to work with, which is the question
// behind "can I plug a Wi-Fi adapter into it".
func reportAdapters(serial string) error {
	adapter, err := robot.ProbeAdapter(serial)
	if err != nil {
		return err
	}

	fmt.Println("\nInterfaces")
	fmt.Println("─────────────────────────────────────────")
	for _, iface := range adapter.Ifaces {
		kind := "wired"
		if iface.Wireless {
			kind = "wireless"
		}

		state := "down"
		if iface.Up {
			state = "up"
		}

		fmt.Printf("  %-10s %-10s %-6s %s\n", iface.Name, kind, state, iface.Address)
	}
	if len(adapter.Ifaces) == 0 {
		fmt.Println("  none reported")
	}

	fmt.Println("\nOn the USB bus")
	fmt.Println("─────────────────────────────────────────")
	for _, device := range adapter.USB {
		name := device.Name
		if name == "" {
			name = "(no name)"
		}

		class := device.Class
		if class == "" {
			class = "?"
		}
		fmt.Printf("  %s:%s  %-14s %s\n", device.Vendor, device.Product, "class "+class, name)
	}
	if len(adapter.USB) == 0 {
		fmt.Println("  nothing")
	}

	// Printed whole rather than only what pusher has a name for. A kernel with
	// a driver this does not recognise is exactly the case somebody needs to
	// see for themselves.
	if registered := robot.Registered(serial); len(registered) > 0 {
		fmt.Println("\nUSB drivers this kernel has")
		fmt.Println("─────────────────────────────────────────")
		fmt.Printf("  %s\n", strings.Join(registered, "  "))
	}

	fmt.Println()
	for _, line := range adapter.Explain() {
		fmt.Println("  " + line)
	}

	if kernel := robot.Kernel(serial); kernel != "" {
		fmt.Println("\nKernel")
		fmt.Println("─────────────────────────────────────────")
		fmt.Printf("  %s\n", kernel)
		if os := robot.OSVersion(serial); os != "" {
			fmt.Printf("  Control Hub OS %s\n", os)
		}
	}

	if lines := robot.Dmesg(serial); len(lines) > 0 {
		fmt.Println("\nWhat the kernel said about USB")
		fmt.Println("─────────────────────────────────────────")
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
	}

	return nil
}
