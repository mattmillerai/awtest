package utils

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/logrusorgru/aurora"
	"strings"
)

// Quiet suppresses informational output when true (set by -quiet flag).
var Quiet bool

// ConcurrentMode buffers output instead of printing inline.
// Set to true during concurrent scans so progress display is not interrupted.
var ConcurrentMode bool

var (
	outputMu     sync.Mutex
	outputBuffer []string
)

// FlushOutput prints all buffered messages and resets concurrent mode.
func FlushOutput() {
	outputMu.Lock()
	defer outputMu.Unlock()
	for _, msg := range outputBuffer {
		fmt.Println(msg)
	}
	outputBuffer = nil
	ConcurrentMode = false
}

const (
	ResetColor   = "\033[0m"
	DisplayColor = "\033[33m"
)

// DetermineSeverity returns the severity based on the error received.
// Returns "hit" for nil errors (accessible services) and "info" for errors.
// Exported for use by formatters package.
func DetermineSeverity(err error) string {
	if err == nil {
		return "hit"
	}
	return "info"
}

// ColorizeMessage creates a colorized message string with the given components.
// Exported for use by formatters package.
func ColorizeMessage(moduleName string, method string, severity string, result string) string {
	moduleNameColored := aurora.BrightGreen(moduleName).String()
	methodColored := aurora.BrightBlue(method).String()
	var severityColored string

	if severity == "high" {
		severityColored = aurora.Red(severity).String()
	} else if severity == "hit" {
		severityColored = aurora.BrightGreen(severity).String()
	} else {
		severityColored = aurora.Blue(severity).String()
	}

	// Splitting the result string at the colon and colorizing the part before it
	parts := strings.SplitN(result, ": ", 2)
	if len(parts) == 2 {
		parts[0] = "\033[35m" + parts[0] + "\033[0m"
		result = parts[0] + ": " + parts[1]
	}

	coloredMessage := fmt.Sprintf("[%s] [%s] [%s] %s", moduleNameColored, methodColored, severityColored, result)
	return coloredMessage
}

func PrintResult(debug bool, moduleName string, method string, result string, err error) {
	if Quiet {
		return
	}
	severity := DetermineSeverity(err)

	if moduleName == "" {
		moduleName = types.DefaultModuleName
	}

	message := ColorizeMessage(moduleName, method, severity, result)

	if ConcurrentMode {
		outputMu.Lock()
		outputBuffer = append(outputBuffer, message)
		outputMu.Unlock()
		return
	}
	fmt.Println(message)
}

func PrintAccessGranted(debug bool, serviceName, resource string, additionalInfo ...interface{}) {
	message := fmt.Sprintf("Access granted. No %s found.", resource)
	if additionalInfo != nil {
		message += " " + fmt.Sprintf(additionalInfo[0].(string), additionalInfo[1:]...)
	}
	PrintResult(debug, "", serviceName, message, nil)
}

func HandleAWSError(debug bool, callName string, err error) error {
	if Quiet {
		// Still detect invalid key errors even in quiet mode
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == types.InvalidAccessKeyId || awsErr.Code() == types.InvalidClientTokenId {
				prettyMsg, exists := types.AwsErrorMessages[awsErr.Code()]
				if !exists {
					prettyMsg = awsErr.Message()
				}
				return &types.InvalidKeyError{Message: prettyMsg}
			}
		}
		return nil
	}
	if awsErr, ok := err.(awserr.Error); ok {
		prettyMsg, exists := types.AwsErrorMessages[awsErr.Code()]
		if !exists {
			prettyMsg = awsErr.Message()
		}

		if awsErr.Code() == "UnauthorizedOperation" ||
			awsErr.Code() == "AccessDenied" ||
			awsErr.Code() == "AccessDeniedException" ||
			awsErr.Code() == "AuthorizationError" {
			if !debug {
				prettyMsg = "Access denied to this service."
			}
		} else if awsErr.Code() == types.InvalidAccessKeyId || awsErr.Code() == types.InvalidClientTokenId {
			PrintResult(debug, "", callName, fmt.Sprintf("Error: %s", prettyMsg), err)
			return &types.InvalidKeyError{Message: prettyMsg}
		}

		PrintResult(debug, "", callName, fmt.Sprintf("Error: %s", prettyMsg), err)
	} else {
		PrintResult(debug, "", callName, fmt.Sprintf("Error: %s", err.Error()), err)
	}
	return nil
}

func ColorizeItem(input string) string {
	return DisplayColor + input + ResetColor
}

func MaskSecret(secret string) string {
	if len(secret) < 6 {
		return "******" // Return all asterisks if the secret is too short
	}
	return secret[:2] + "****" + secret[len(secret)-2:]
}

func UnmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
