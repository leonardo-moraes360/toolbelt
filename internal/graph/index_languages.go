package graph

import "regexp"

func init() {
	for i := range regexLanguageSpecs {
		RegisterIndexer(&regexLanguageSpecs[i])
	}
}

var regexLanguageSpecs = []regexLang{
	{
		name: "python",
		exts: extSet(".py"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*class\s+([A-Za-z_]\w*)\s*[:(]`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*import\s+([A-Za-z_][\w.]*)`),
			mustRE(`(?m)^\s*from\s+([A-Za-z_][\w.]*)\s+import\s+`),
		},
		skipTests: true,
	},
	{
		name: "java",
		exts: extSet(".java"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:public|private|protected|static|final|synchronized|native|abstract|default|[\s])*[\w.<>,\[\]]+\s+([A-Za-z_]\w*)\s*\(`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:public|private|protected|abstract|final|static|\s)*class\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:public|private|protected|abstract|static|\s)*interface\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:public|private|protected|\s)*enum\s+([A-Za-z_]\w*)`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*import\s+(?:static\s+)?([A-Za-z_][\w.*]*);`),
		},
		skipTests: true,
	},
	{
		name: "kotlin",
		exts: extSet(".kt", ".kts"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:override\s+|suspend\s+|inline\s+|private\s+|public\s+|internal\s+|protected\s+)*fun\s+(?:[A-Za-z_]\w*\.)?([A-Za-z_]\w*)\s*[<(]`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:data\s+|sealed\s+|abstract\s+|open\s+|final\s+|inner\s+)*class\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:sealed\s+)?interface\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*object\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*enum\s+class\s+([A-Za-z_]\w*)`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*import\s+([A-Za-z_][\w.]*)`),
		},
		skipTests: true,
	},
	{
		name: "csharp",
		exts: extSet(".cs"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:public|private|protected|internal|static|virtual|override|async|partial|[\s])*[\w.<>,\[\]]+\s+([A-Za-z_]\w*)\s*\(`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:public|private|protected|internal|static|abstract|sealed|partial|\s)*class\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:public|private|protected|internal|\s)*interface\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:public|private|protected|internal|\s)*enum\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:public|private|protected|internal|\s)*record\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:public|private|protected|internal|\s)*struct\s+([A-Za-z_]\w*)`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*using\s+([A-Za-z_][\w.]*)\s*;`),
		},
		skipTests: true,
	},
	{
		name: "cpp",
		exts: extSet(".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^[\w:\s\*&<>,\[\]]+\s+([A-Za-z_]\w*)\s*\([^;{}]*\)\s*(?:const\s*)?(?:override\s*)?\{`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:template\s*<[^>]+>\s*)?(?:class|struct)\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:enum(?:\s+class)?)\s+([A-Za-z_]\w*)`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*#\s*include\s*[<"]([^>"]+)[>"]`),
		},
		skipTests: true,
	},
	{
		name: "rust",
		exts: extSet(".rs"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:const\s+)?fn\s+([A-Za-z_]\w*)\s*[<(]`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?(?:struct|enum|trait|type)\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?mod\s+([A-Za-z_]\w*)`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*use\s+([A-Za-z_][\w:]*(?:::\*)?)`),
		},
		skipTests: true,
	},
	{
		name: "php",
		exts: extSet(".php"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:public|private|protected|static|final|abstract|\s)*function\s+&?([A-Za-z_]\w*)\s*\(`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:abstract\s+|final\s+)?class\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*interface\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*trait\s+([A-Za-z_]\w*)`),
			mustRE(`(?m)^\s*enum\s+([A-Za-z_]\w*)`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*use\s+([A-Za-z_\\][\w\\]*)\s*;`),
			mustRE(`(?m)^\s*(?:require|include)(?:_once)?\s*\(?\s*['"]([^'"]+)['"]`),
		},
		skipTests: true,
	},
	{
		name: "ruby",
		exts: extSet(".rb"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*def\s+(?:self\.)?([A-Za-z_]\w*[!?=]?)`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*class\s+([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)`),
			mustRE(`(?m)^\s*module\s+([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*require(?:_relative)?\s+['"]([^'"]+)['"]`),
			mustRE(`(?m)^\s*load\s+['"]([^'"]+)['"]`),
		},
		skipTests: true,
	},
	{
		name: "swift",
		exts: extSet(".swift"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:public|private|internal|open|fileprivate|static|class|override|mutating|\s)*func\s+([A-Za-z_]\w*)\s*[<(]`),
		},
		classRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:public|private|internal|open|fileprivate|final|\s)*(?:class|struct|enum|protocol|actor)\s+([A-Za-z_]\w*)`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*import\s+([A-Za-z_]\w*)`),
		},
		skipTests: true,
	},
	{
		name: "shell",
		exts: extSet(".sh", ".bash", ".zsh", ".ksh"),
		funcRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:function\s+)?([A-Za-z_]\w*)\s*\(\s*\)\s*\{`),
			mustRE(`(?m)^\s*function\s+([A-Za-z_]\w*)\s*\{`),
		},
		importRes: []*regexp.Regexp{
			mustRE(`(?m)^\s*(?:source|\.)\s+['"]?([^\s'"]+)`),
		},
		skipTests: false,
	},
}
