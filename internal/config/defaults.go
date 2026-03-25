package config

func DefaultSafetyConfig() SafetyConfig {
	return SafetyConfig{
		MaxFileWriteBytes: 1 << 20, // 1 MiB
		AllowedCommands: []AllowedCommand{
			{Cmd: "go", AllowAll: true},
			{Cmd: "grep", AllowAll: true},
			{Cmd: "find", AllowAll: true},
			{Cmd: "cat", AllowAll: true},
			{Cmd: "head", AllowAll: true},
			{Cmd: "tail", AllowAll: true},
			{Cmd: "ls", AllowAll: true},
			{Cmd: "wc", AllowAll: true},
			{Cmd: "tree", AllowAll: true},
			{Cmd: "make", AllowAll: true},
			{Cmd: "git", ArgsPrefix: []string{"diff"}},
			{Cmd: "git", ArgsPrefix: []string{"log"}},
			{Cmd: "git", ArgsPrefix: []string{"show"}},
			{Cmd: "git", ArgsPrefix: []string{"status"}},
		},
	}
}
