// Command hashpw turns a plaintext password into the bcrypt hash that goes
// into the *_PASSWORD_HASH environment variables consumed by internal/auth.
package main

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	fmt.Print("Password: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(os.Stderr, "hashpw: failed to read password:", err)
		os.Exit(1)
	}

	password := trimNewline(line)
	if password == "" {
		fmt.Fprintln(os.Stderr, "hashpw: password must not be empty")
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hashpw: failed to hash password:", err)
		os.Exit(1)
	}

	fmt.Println(string(hash))
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
