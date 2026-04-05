package themes

// GetTheme returns a named theme, falling back to dark.
func GetTheme(name ThemeName) Theme {
	switch name {
	case ThemeLight:
		return lightTheme()
	case ThemeDarkAnsi:
		return darkAnsiTheme()
	case ThemeLightAnsi:
		return lightAnsiTheme()
	case ThemeDarkDaltonized:
		return darkDaltonizedTheme()
	case ThemeLightDaltonized:
		return lightDaltonizedTheme()
	default:
		return darkTheme()
	}
}

func darkTheme() Theme {
	return Theme{
		Name:                ThemeDark,
		AutoAccept:          "rgb(78,186,101)",
		BashBorder:          "rgb(253,93,177)",
		Claude:              "rgb(215,119,87)",
		ClaudeShimmer:       "rgb(253,165,122)",
		Permission:          "rgb(177,185,249)",
		PermissionShimmer:   "rgb(215,219,255)",
		PlanMode:            "rgb(255,193,7)",
		PromptBorder:        "rgb(136,136,136)",
		PromptBorderShimmer: "rgb(200,200,200)",
		Text:                "rgb(255,255,255)",
		InverseText:         "rgb(0,0,0)",
		Inactive:            "rgb(153,153,153)",
		InactiveShimmer:     "rgb(200,200,200)",
		Subtle:              "rgb(80,80,80)",
		Suggestion:          "rgb(100,149,237)",
		Background:          "rgb(0,0,0)",
		Success:             "rgb(78,186,101)",
		Error:               "rgb(255,107,128)",
		Warning:             "rgb(255,193,7)",
		WarningShimmer:      "rgb(255,223,100)",
		Merged:              "rgb(177,185,249)",
		DiffAdded:           "rgb(34,92,43)",
		DiffRemoved:         "rgb(122,41,54)",
		DiffAddedDimmed:     "rgb(20,60,25)",
		DiffRemovedDimmed:   "rgb(80,25,35)",
		DiffAddedWord:       "rgb(50,140,60)",
		DiffRemovedWord:     "rgb(180,55,75)",
		LobsterBody:         "rgb(215,119,87)",
		LobsterBackground:   "rgb(0,0,0)",
		UserMsgBg:           "rgb(25,25,35)",
		UserMsgBgHover:      "rgb(35,35,50)",
		BashMsgBg:           "rgb(35,20,25)",
		MemoryBg:            "rgb(20,30,35)",
		SelectionBg:         "rgb(40,40,60)",
		MsgActionsBg:        "rgb(30,30,45)",
		RateLimitFill:       "rgb(215,119,87)",
		RateLimitEmpty:      "rgb(60,60,60)",
		FastMode:            "rgb(255,193,7)",
		FastModeShimmer:     "rgb(255,223,100)",
		BriefLabelYou:       "rgb(120,120,120)",
		BriefLabelClaude:    "rgb(215,119,87)",
		AgentRed:            "rgb(255,107,128)",
		AgentBlue:           "rgb(100,149,237)",
		AgentGreen:          "rgb(78,186,101)",
		AgentYellow:         "rgb(255,193,7)",
		AgentPurple:         "rgb(177,130,255)",
		AgentOrange:         "rgb(255,165,79)",
		AgentPink:           "rgb(253,93,177)",
		AgentCyan:           "rgb(80,200,200)",
	}
}

func lightTheme() Theme {
	return Theme{
		Name:                ThemeLight,
		AutoAccept:          "rgb(22,130,50)",
		BashBorder:          "rgb(200,50,130)",
		Claude:              "rgb(178,82,52)",
		ClaudeShimmer:       "rgb(215,119,87)",
		Permission:          "rgb(87,105,247)",
		PermissionShimmer:   "rgb(130,140,255)",
		PlanMode:            "rgb(180,130,0)",
		PromptBorder:        "rgb(136,136,136)",
		PromptBorderShimmer: "rgb(80,80,80)",
		Text:                "rgb(0,0,0)",
		InverseText:         "rgb(255,255,255)",
		Inactive:            "rgb(120,120,120)",
		InactiveShimmer:     "rgb(80,80,80)",
		Subtle:              "rgb(200,200,200)",
		Suggestion:          "rgb(50,100,200)",
		Background:          "rgb(255,255,255)",
		Success:             "rgb(22,130,50)",
		Error:               "rgb(200,40,60)",
		Warning:             "rgb(180,130,0)",
		WarningShimmer:      "rgb(220,170,30)",
		Merged:              "rgb(87,105,247)",
		DiffAdded:           "rgb(210,255,210)",
		DiffRemoved:         "rgb(255,210,210)",
		DiffAddedDimmed:     "rgb(230,255,230)",
		DiffRemovedDimmed:   "rgb(255,230,230)",
		DiffAddedWord:       "rgb(130,230,130)",
		DiffRemovedWord:     "rgb(255,130,130)",
		LobsterBody:         "rgb(178,82,52)",
		LobsterBackground:   "rgb(255,255,255)",
		UserMsgBg:           "rgb(240,240,250)",
		UserMsgBgHover:      "rgb(230,230,245)",
		BashMsgBg:           "rgb(250,235,240)",
		MemoryBg:            "rgb(235,245,250)",
		SelectionBg:         "rgb(220,220,240)",
		MsgActionsBg:        "rgb(235,235,250)",
		RateLimitFill:       "rgb(178,82,52)",
		RateLimitEmpty:      "rgb(220,220,220)",
		FastMode:            "rgb(180,130,0)",
		FastModeShimmer:     "rgb(220,170,30)",
		BriefLabelYou:       "rgb(150,150,150)",
		BriefLabelClaude:    "rgb(178,82,52)",
		AgentRed:            "rgb(200,40,60)",
		AgentBlue:           "rgb(50,100,200)",
		AgentGreen:          "rgb(22,130,50)",
		AgentYellow:         "rgb(180,130,0)",
		AgentPurple:         "rgb(120,80,200)",
		AgentOrange:         "rgb(200,120,30)",
		AgentPink:           "rgb(200,50,130)",
		AgentCyan:           "rgb(30,150,150)",
	}
}

func darkAnsiTheme() Theme {
	return Theme{
		Name:                ThemeDarkAnsi,
		AutoAccept:          "ansi:green",
		BashBorder:          "ansi:magentaBright",
		Claude:              "ansi:yellowBright",
		ClaudeShimmer:       "ansi:yellowBright",
		Permission:          "ansi:blueBright",
		PermissionShimmer:   "ansi:blueBright",
		PlanMode:            "ansi:yellowBright",
		PromptBorder:        "ansi:white",
		PromptBorderShimmer: "ansi:whiteBright",
		Text:                "ansi:whiteBright",
		InverseText:         "ansi:black",
		Inactive:            "ansi:white",
		InactiveShimmer:     "ansi:whiteBright",
		Subtle:              "ansi:blackBright",
		Suggestion:          "ansi:cyanBright",
		Background:          "ansi:black",
		Success:             "ansi:greenBright",
		Error:               "ansi:redBright",
		Warning:             "ansi:yellowBright",
		WarningShimmer:      "ansi:yellowBright",
		Merged:              "ansi:blueBright",
		DiffAdded:           "ansi:green",
		DiffRemoved:         "ansi:red",
		DiffAddedDimmed:     "ansi:green",
		DiffRemovedDimmed:   "ansi:red",
		DiffAddedWord:       "ansi:greenBright",
		DiffRemovedWord:     "ansi:redBright",
		LobsterBody:         "ansi:yellowBright",
		LobsterBackground:   "ansi:black",
		UserMsgBg:           "ansi:black",
		UserMsgBgHover:      "ansi:blackBright",
		BashMsgBg:           "ansi:black",
		MemoryBg:            "ansi:black",
		SelectionBg:         "ansi:blackBright",
		MsgActionsBg:        "ansi:blackBright",
		RateLimitFill:       "ansi:yellowBright",
		RateLimitEmpty:      "ansi:blackBright",
		FastMode:            "ansi:yellowBright",
		FastModeShimmer:     "ansi:yellowBright",
		BriefLabelYou:       "ansi:white",
		BriefLabelClaude:    "ansi:yellowBright",
		AgentRed:            "ansi:redBright",
		AgentBlue:           "ansi:blueBright",
		AgentGreen:          "ansi:greenBright",
		AgentYellow:         "ansi:yellowBright",
		AgentPurple:         "ansi:magentaBright",
		AgentOrange:         "ansi:yellowBright",
		AgentPink:           "ansi:magentaBright",
		AgentCyan:           "ansi:cyanBright",
	}
}

func lightAnsiTheme() Theme {
	return Theme{
		Name:                ThemeLightAnsi,
		AutoAccept:          "ansi:green",
		BashBorder:          "ansi:magenta",
		Claude:              "ansi:yellow",
		ClaudeShimmer:       "ansi:yellow",
		Permission:          "ansi:blue",
		PermissionShimmer:   "ansi:blue",
		PlanMode:            "ansi:yellow",
		PromptBorder:        "ansi:blackBright",
		PromptBorderShimmer: "ansi:black",
		Text:                "ansi:black",
		InverseText:         "ansi:whiteBright",
		Inactive:            "ansi:blackBright",
		InactiveShimmer:     "ansi:black",
		Subtle:              "ansi:white",
		Suggestion:          "ansi:cyan",
		Background:          "ansi:whiteBright",
		Success:             "ansi:green",
		Error:               "ansi:red",
		Warning:             "ansi:yellow",
		WarningShimmer:      "ansi:yellow",
		Merged:              "ansi:blue",
		DiffAdded:           "ansi:green",
		DiffRemoved:         "ansi:red",
		DiffAddedDimmed:     "ansi:green",
		DiffRemovedDimmed:   "ansi:red",
		DiffAddedWord:       "ansi:greenBright",
		DiffRemovedWord:     "ansi:redBright",
		LobsterBody:         "ansi:yellow",
		LobsterBackground:   "ansi:whiteBright",
		UserMsgBg:           "ansi:whiteBright",
		UserMsgBgHover:      "ansi:white",
		BashMsgBg:           "ansi:whiteBright",
		MemoryBg:            "ansi:whiteBright",
		SelectionBg:         "ansi:white",
		MsgActionsBg:        "ansi:white",
		RateLimitFill:       "ansi:yellow",
		RateLimitEmpty:      "ansi:white",
		FastMode:            "ansi:yellow",
		FastModeShimmer:     "ansi:yellow",
		BriefLabelYou:       "ansi:blackBright",
		BriefLabelClaude:    "ansi:yellow",
		AgentRed:            "ansi:red",
		AgentBlue:           "ansi:blue",
		AgentGreen:          "ansi:green",
		AgentYellow:         "ansi:yellow",
		AgentPurple:         "ansi:magenta",
		AgentOrange:         "ansi:yellow",
		AgentPink:           "ansi:magenta",
		AgentCyan:           "ansi:cyan",
	}
}

func darkDaltonizedTheme() Theme {
	t := darkTheme()
	t.Name = ThemeDarkDaltonized
	t.Success = "rgb(0,176,240)"
	t.Error = "rgb(255,176,0)"
	t.Warning = "rgb(255,255,100)"
	t.DiffAdded = "rgb(0,60,120)"
	t.DiffRemoved = "rgb(120,60,0)"
	t.DiffAddedDimmed = "rgb(0,40,80)"
	t.DiffRemovedDimmed = "rgb(80,40,0)"
	t.DiffAddedWord = "rgb(0,120,200)"
	t.DiffRemovedWord = "rgb(200,120,0)"
	t.AgentRed = "rgb(255,176,0)"
	t.AgentGreen = "rgb(0,176,240)"
	return t
}

func lightDaltonizedTheme() Theme {
	t := lightTheme()
	t.Name = ThemeLightDaltonized
	t.Success = "rgb(0,130,200)"
	t.Error = "rgb(200,130,0)"
	t.Warning = "rgb(200,180,0)"
	t.DiffAdded = "rgb(210,235,255)"
	t.DiffRemoved = "rgb(255,235,210)"
	t.DiffAddedDimmed = "rgb(230,245,255)"
	t.DiffRemovedDimmed = "rgb(255,245,230)"
	t.DiffAddedWord = "rgb(100,180,240)"
	t.DiffRemovedWord = "rgb(240,180,100)"
	t.AgentRed = "rgb(200,130,0)"
	t.AgentGreen = "rgb(0,130,200)"
	return t
}
