package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/feature"
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
	fmt.Println("  pusher                Build and deploy to the robot")
	fmt.Println("  pusher connect        Join the robot Wi-Fi and connect adb")
	fmt.Println("  pusher exit           Disconnect adb and go back to your Wi-Fi")
	fmt.Println("  pusher dc             Disconnect adb only (alias: disconnect)")
	fmt.Println("  pusher settings       Robot profiles and preferences (alias: config)")
	fmt.Println("  pusher slim           Shrink the APK so deploys transfer less")
	fmt.Println("    pusher slim --undo       Put the gradle files back")
	fmt.Println("  --ignore-warnings     Carry on past a check that would stop a command")
	fmt.Println("  pusher hwconfig       Hardware config menu and editor (alias: hw)")
	fmt.Println("    pusher hwconfig list     Print what the robot and the project have")
	fmt.Println("    pusher hwconfig pull     Copy the robot's configs into your project")
	fmt.Println("    pusher hwconfig push X   Copy X back to the robot")
	fmt.Println("  pusher dash diff      What the robot holds that your code does not")
	fmt.Println("  pusher prepare        Cache dependencies while you have internet")
	if feature.Revealed() {
		fmt.Println("  pusher visualiser     Draw the path an auto drove (alias: vis)")
	}
	fmt.Println("  pusher dev            Measure what a deploy costs (see the warning)")
	fmt.Println("  pusher update         Update pusher itself to the latest release")
	fmt.Println("    pusher update --check    Say what is available, install nothing")
	fmt.Println("  pusher --version      Show version information")
	fmt.Println("  pusher help           Show this help")
	fmt.Println("")
	fmt.Println("Pusher Extreme:")
	fmt.Println("  Reloads your OpModes onto a running robot instead of installing an")
	fmt.Println("  APK: under a second rather than around forty. Set it up in")
	fmt.Println("  'pusher settings' -> Pusher Extreme, which also undoes it.")
	fmt.Println("  While it is set up your team code is not part of the APK.")
	fmt.Println("")
	fmt.Println("pusher dev:")
	fmt.Println("  Measuring tools for working on pusher itself. It deploys to the")
	fmt.Println("  robot repeatedly and reinstalls the app several times. If you do")
	fmt.Println("  not already know why you want it, you do not want it.")
	fmt.Println("")
	fmt.Println("Deploying:")
	fmt.Println("  A hub on USB is used automatically and your Wi-Fi is left alone.")
	fmt.Println("  Otherwise pusher builds first, hops to the robot, deploys, and")
	fmt.Println("  puts you back on the network you started on.")
}
