package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const asciiArt = `
 ____            _               
|  _ \ _   _ ___| |__   ___ _ __ 
| |_) | | | / __| '_ \ / _ \ '__|
|  __/| |_| \__ \ | | |  __/ |   
|_|    \__,_|___/_| |_|\___|_|   
                                  
FTC Robot Deployment Tool
`

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show help information",
	Run:   runHelp,
}

func runHelp(cmd *cobra.Command, args []string) {
	fmt.Println(asciiArt)
	fmt.Println("Made with love by:")
	fmt.Println("  Credits: Andrei Banu")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  pusher                Connect, build, and deploy")
	fmt.Println("  pusher dc             Disconnect adb")
	fmt.Println("  pusher disconnect     Alias for dc")
	fmt.Println("  pusher exit           Disconnect adb and restore Wi-Fi")
	fmt.Println("  pusher profile        Manage robot profiles")
	fmt.Println("    pusher profile list      List all profiles")
	fmt.Println("    pusher profile add       Add a new profile")
	fmt.Println("    pusher profile edit      Edit an existing profile")
	fmt.Println("    pusher profile use       Set default profile")
	fmt.Println("  pusher help           Show this help")
	fmt.Println()
}
