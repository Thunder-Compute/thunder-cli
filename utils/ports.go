package utils

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	minPort         = 1
	maxPort         = 65535
	reservedSSHPort = 22
	minRangeDisplay = 5
)

// ParsePorts parses a comma-separated list of ports and/or ranges (e.g. "8080, 9000-9005").
// It returns de-duplicated ports in the order they appear.
func ParsePorts(input string) ([]int, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}

	parts := strings.Split(input, ",")
	ports := make([]int, 0, len(parts))
	seen := make(map[int]bool)

	for _, raw := range parts {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}

		if strings.Contains(token, "-") {
			rangeParts := strings.Split(token, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range: %s", token)
			}

			startStr := strings.TrimSpace(rangeParts[0])
			endStr := strings.TrimSpace(rangeParts[1])
			if startStr == "" || endStr == "" {
				return nil, fmt.Errorf("invalid port range: %s", token)
			}

			start, err := strconv.Atoi(startStr)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", startStr)
			}
			end, err := strconv.Atoi(endStr)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", endStr)
			}
			if start > end {
				return nil, fmt.Errorf("invalid port range: %s", token)
			}
			if err := validatePortBounds(start); err != nil {
				return nil, err
			}
			if err := validatePortBounds(end); err != nil {
				return nil, err
			}
			if start <= reservedSSHPort && reservedSSHPort <= end {
				return nil, fmt.Errorf("port %d is reserved for SSH", reservedSSHPort)
			}

			for p := start; p <= end; p++ {
				if !seen[p] {
					ports = append(ports, p)
					seen[p] = true
				}
			}
			continue
		}

		port, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", token)
		}
		if err := validatePort(port); err != nil {
			return nil, err
		}
		if !seen[port] {
			ports = append(ports, port)
			seen[port] = true
		}
	}

	return ports, nil
}

// FormatPorts returns a human-friendly string. Consecutive runs of at least
// five ports are collapsed into ranges (e.g. "8000-8006").
func FormatPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}

	sorted := append([]int(nil), ports...)
	sort.Ints(sorted)

	unique := make([]int, 0, len(sorted))
	for _, p := range sorted {
		if len(unique) == 0 || unique[len(unique)-1] != p {
			unique = append(unique, p)
		}
	}

	var parts []string
	start := unique[0]
	prev := unique[0]

	flush := func(s, e int) {
		if e-s+1 >= minRangeDisplay {
			parts = append(parts, fmt.Sprintf("%d-%d", s, e))
			return
		}
		for p := s; p <= e; p++ {
			parts = append(parts, strconv.Itoa(p))
		}
	}

	for _, p := range unique[1:] {
		if p == prev+1 {
			prev = p
			continue
		}
		flush(start, prev)
		start = p
		prev = p
	}
	flush(start, prev)

	return strings.Join(parts, ", ")
}

func validatePortBounds(port int) error {
	if port < minPort || port > maxPort {
		return fmt.Errorf("port %d out of range (1-65535)", port)
	}
	return nil
}

func validatePort(port int) error {
	if err := validatePortBounds(port); err != nil {
		return err
	}
	if port == reservedSSHPort {
		return fmt.Errorf("port %d is reserved for SSH", reservedSSHPort)
	}
	return nil
}
