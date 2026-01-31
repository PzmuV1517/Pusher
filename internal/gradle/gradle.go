package gradle

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectWrapper finds the Gradle wrapper in the current directory
func DetectWrapper() (string, error) {
	// Check current directory
	wrapper := "./gradlew"
	if _, err := os.Stat(wrapper); err == nil {
		return wrapper, nil
	}

	// Check parent directories (up to 3 levels)
	for i := 0; i < 3; i++ {
		wrapper = filepath.Join(strings.Repeat("../", i+1), "gradlew")
		if _, err := os.Stat(wrapper); err == nil {
			absPath, _ := filepath.Abs(wrapper)
			return absPath, nil
		}
	}

	return "", fmt.Errorf("gradlew not found in current directory or parent directories")
}

// Build runs the Gradle build and install tasks
func Build(wrapper string, outputWriter io.Writer) error {
	if _, err := os.Stat(wrapper); err != nil {
		return fmt.Errorf("gradle wrapper not found: %s", wrapper)
	}

	// Make sure gradlew is executable
	if err := os.Chmod(wrapper, 0755); err != nil {
		return fmt.Errorf("failed to make gradlew executable: %w", err)
	}

	// Use Gradle in offline mode so it relies on dependencies
	// already downloaded by Android Studio, avoiding network
	// access while connected to the robot's Wi-Fi.
	cmd := exec.Command(wrapper, "assembleDebug", "installDebug", "--offline")

	// Get working directory from wrapper path
	wrapperDir := filepath.Dir(wrapper)
	cmd.Dir = wrapperDir

	// If JAVA_HOME is not set, try to use Android Studio's bundled JDK.
	// This avoids the "Unable to locate a Java Runtime" error on macOS.
	if os.Getenv("JAVA_HOME") == "" {
		candidate := "/Applications/Android Studio.app/Contents/jbr/Contents/Home"
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			if cmd.Env == nil {
				cmd.Env = os.Environ()
			}
			cmd.Env = append(cmd.Env, "JAVA_HOME="+candidate)
			cmd.Env = append(cmd.Env, "PATH="+filepath.Join(candidate, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}

	// Capture both stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start gradle: %w", err)
	}

	// Stream output
	done := make(chan bool)
	go streamOutput(stdout, outputWriter, done)
	go streamOutput(stderr, outputWriter, done)

	// Wait for both streams to complete
	<-done
	<-done

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("gradle build failed: %w", err)
	}

	return nil
}

// BuildOnline runs the Gradle build and install tasks without offline mode.
// Use this when on normal internet (e.g. via `pusher prepare`) so that
// dependencies are downloaded and cached for offline builds later.
func BuildOnline(wrapper string, outputWriter io.Writer) error {
	if _, err := os.Stat(wrapper); err != nil {
		return fmt.Errorf("gradle wrapper not found: %s", wrapper)
	}

	if err := os.Chmod(wrapper, 0755); err != nil {
		return fmt.Errorf("failed to make gradlew executable: %w", err)
	}

	cmd := exec.Command(wrapper, "assembleDebug", "installDebug")

	wrapperDir := filepath.Dir(wrapper)
	cmd.Dir = wrapperDir

	if os.Getenv("JAVA_HOME") == "" {
		candidate := "/Applications/Android Studio.app/Contents/jbr/Contents/Home"
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			if cmd.Env == nil {
				cmd.Env = os.Environ()
			}
			cmd.Env = append(cmd.Env, "JAVA_HOME="+candidate)
			cmd.Env = append(cmd.Env, "PATH="+filepath.Join(candidate, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start gradle: %w", err)
	}

	done := make(chan bool)
	go streamOutput(stdout, outputWriter, done)
	go streamOutput(stderr, outputWriter, done)

	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("gradle build failed: %w", err)
	}

	return nil
}

func streamOutput(reader io.Reader, writer io.Writer, done chan bool) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Fprintln(writer, scanner.Text())
	}
	done <- true
}
