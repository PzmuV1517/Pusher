package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const asciiArt = `
From Team #14270

 ██████╗ ██╗   ██╗ █████╗ ███╗   ██╗████████╗██╗   ██╗███╗   ███╗
██╔═══██╗██║   ██║██╔══██╗████╗  ██║╚══██╔══╝██║   ██║████╗ ████║
██║   ██║██║   ██║███████║██╔██╗ ██║   ██║   ██║   ██║██╔████╔██║
██║▄▄ ██║██║   ██║██╔══██║██║╚██╗██║   ██║   ██║   ██║██║╚██╔╝██║
╚██████╔╝╚██████╔╝██║  ██║██║ ╚████║   ██║   ╚██████╔╝██║ ╚═╝ ██║
 ╚══▀▀═╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝    ╚═════╝ ╚═╝     ╚═╝

 ██████╗  ██████╗ ██████╗  ██████╗ ████████╗██╗ ██████╗███████╗
██╔══██╗██╔═══██╗██╔══██╗██╔═══██╗╚══██╔══╝██║██╔════╝██╔════╝
██████╔╝██║   ██║██████╔╝██║   ██║   ██║   ██║██║     ███████╗
██╔══██╗██║   ██║██╔══██╗██║   ██║   ██║   ██║██║     ╚════██║
██║  ██║╚██████╔╝██████╔╝╚██████╔╝   ██║   ██║╚██████╗███████║
╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝    ╚═╝   ╚═╝ ╚═════╝╚══════╝ 

`

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show help information",
	Run:   runHelp,
}

func runHelp(cmd *cobra.Command, args []string) {
	fmt.Print(asciiArt)
	fmt.Println("Made with love by:")
	fmt.Println("	Andrei \"PzmuV1517\" Banu")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  pusher                Connect, build, and deploy")
	fmt.Println("  pusher connect        Connect to robot Wi-Fi only")
	fmt.Println("  pusher --version      Show version information")
	fmt.Println("  pusher dc             Disconnect adb")
	fmt.Println("  pusher disconnect     Alias for dc")
	fmt.Println("  pusher exit           Disconnect adb and restore Wi-Fi")
	fmt.Println("  pusher profile        Manage robot profiles")
	fmt.Println("    pusher profile list      List all profiles")
	fmt.Println("    pusher profile add       Add a new profile")
	fmt.Println("    pusher profile edit      Edit an existing profile")
	fmt.Println("    pusher profile use       Set default profile")
	fmt.Println("  pusher help           Show this help")

}
