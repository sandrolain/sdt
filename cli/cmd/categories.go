package cmd

import "github.com/spf13/cobra"

type commandCategory struct {
	id    string
	title string
}

const (
	categoryProjectAgents          = "project-agents"
	categoryKnowledgeLearning      = "knowledge-learning"
	categoryFilesData              = "files-data"
	categoryDevelopmentMaintenance = "development-maintenance"
	categorySDTDiagnostics         = "sdt-diagnostics"
)

var categoryTitles = map[string]string{
	categoryProjectAgents:          "Project / agents",
	categoryKnowledgeLearning:      "Knowledge / learning",
	categoryFilesData:              "Files / data",
	categoryDevelopmentMaintenance: "Development / maintenance",
	categorySDTDiagnostics:         "SDT diagnostics",
}

// Keys are the registered Cobra commands, so renaming or removing a command
// updates discovery from the actual command catalogue instead of a parallel
// list of string names.
var commandCategories map[*cobra.Command]commandCategory

func init() {
	commandCategories = map[*cobra.Command]commandCategory{
		agentCmd:   {id: categoryProjectAgents},
		configCmd:  {id: categoryProjectAgents},
		contextCmd: {id: categoryProjectAgents},

		chunkCmd:    {id: categoryKnowledgeLearning},
		learnCmd:    {id: categoryKnowledgeLearning},
		materialCmd: {id: categoryKnowledgeLearning},
		researchCmd: {id: categoryKnowledgeLearning},

		b32Cmd:       {id: categoryFilesData},
		b64Cmd:       {id: categoryFilesData},
		b64UrlCmd:    {id: categoryFilesData},
		convCmd:      {id: categoryFilesData},
		crawldownCmd: {id: categoryFilesData},
		diffCmd:      {id: categoryFilesData},
		doc2mdCmd:    {id: categoryFilesData},
		editCmd:      {id: categoryFilesData},
		envCmd:       {id: categoryFilesData},
		extractCmd:   {id: categoryFilesData},
		figureCmd:    {id: categoryFilesData},
		findCmd:      {id: categoryFilesData},
		grepCmd:      {id: categoryFilesData},
		gunzipCmd:    {id: categoryFilesData},
		gzipCmd:      {id: categoryFilesData},
		hexCmd:       {id: categoryFilesData},
		htmlCmd:      {id: categoryFilesData},
		imageCmd:     {id: categoryFilesData},
		jsonCmd:      {id: categoryFilesData},
		lsCmd:        {id: categoryFilesData},
		qCmd:         {id: categoryFilesData},
		fileReadCmd:  {id: categoryFilesData},
		regeCmd:      {id: categoryFilesData},
		stringCmd:    {id: categoryFilesData},
		urlCmd:       {id: categoryFilesData},
		fileWriteCmd: {id: categoryFilesData},

		bcryptCmd:   {id: categoryDevelopmentMaintenance},
		bytesCmd:    {id: categoryDevelopmentMaintenance},
		certCmd:     {id: categoryDevelopmentMaintenance},
		colorCmd:    {id: categoryDevelopmentMaintenance},
		dnsCmd:      {id: categoryDevelopmentMaintenance},
		hmacCmd:     {id: categoryDevelopmentMaintenance},
		httpCmd:     {id: categoryDevelopmentMaintenance},
		ipInfoCmd:   {id: categoryDevelopmentMaintenance},
		jwtCmd:      {id: categoryDevelopmentMaintenance},
		keypairCmd:  {id: categoryDevelopmentMaintenance},
		md5Cmd:      {id: categoryDevelopmentMaintenance},
		modelsCmd:   {id: categoryDevelopmentMaintenance},
		passwordCmd: {id: categoryDevelopmentMaintenance},
		portCmd:     {id: categoryDevelopmentMaintenance},
		qrcodeCmd:   {id: categoryDevelopmentMaintenance},
		sha1Cmd:     {id: categoryDevelopmentMaintenance},
		sha256Cmd:   {id: categoryDevelopmentMaintenance},
		sha384Cmd:   {id: categoryDevelopmentMaintenance},
		sha512Cmd:   {id: categoryDevelopmentMaintenance},
		signRootCmd: {id: categoryDevelopmentMaintenance},
		templateCmd: {id: categoryDevelopmentMaintenance},
		timeCmd:     {id: categoryDevelopmentMaintenance},
		tokensCmd:   {id: categoryDevelopmentMaintenance},
		totpCmd:     {id: categoryDevelopmentMaintenance},
		truncateCmd: {id: categoryDevelopmentMaintenance},
		uidCmd:      {id: categoryDevelopmentMaintenance},
		verifyCmd:   {id: categoryDevelopmentMaintenance},
		vmanCmd:     {id: categoryDevelopmentMaintenance},

		docCmd:      {id: categorySDTDiagnostics},
		gainCmd:     {id: categorySDTDiagnostics},
		manifestCmd: {id: categorySDTDiagnostics},
		schemaCmd:   {id: categorySDTDiagnostics},
		versionCmd:  {id: categorySDTDiagnostics},
	}
}

var commandCategoryOrder = []string{
	categoryProjectAgents,
	categoryKnowledgeLearning,
	categoryFilesData,
	categoryDevelopmentMaintenance,
	categorySDTDiagnostics,
}

func configureCommandGroups(root *cobra.Command) {
	wantGroups := []*cobra.Group{
		{ID: categoryProjectAgents, Title: categoryTitles[categoryProjectAgents]},
		{ID: categoryKnowledgeLearning, Title: categoryTitles[categoryKnowledgeLearning]},
		{ID: categoryFilesData, Title: categoryTitles[categoryFilesData]},
		{ID: categoryDevelopmentMaintenance, Title: categoryTitles[categoryDevelopmentMaintenance]},
		{ID: categorySDTDiagnostics, Title: categoryTitles[categorySDTDiagnostics]},
	}
	existing := make(map[string]bool, len(root.Groups()))
	for _, group := range root.Groups() {
		existing[group.ID] = true
	}
	for _, group := range wantGroups {
		if !existing[group.ID] {
			root.AddGroup(group)
		}
	}
	for _, command := range root.Commands() {
		if category, ok := categoryForCommandRef(command); ok {
			command.GroupID = category.id
		}
	}
}

func categoryForCommandRef(command *cobra.Command) (commandCategory, bool) {
	category, ok := commandCategories[command]
	if ok {
		category.title = categoryTitles[category.id]
		return category, true
	}
	if _, ok := cobraBuiltins[command.Name()]; ok {
		return commandCategory{id: categorySDTDiagnostics, title: categoryTitles[categorySDTDiagnostics]}, true
	}
	return commandCategory{}, false
}

func categoryTitle(id string) string {
	if title, ok := categoryTitles[id]; ok {
		return title
	}
	return id
}

func categoryForCommand(name string) (commandCategory, bool) {
	for command := range commandCategories {
		if command.Name() == name {
			return categoryForCommandRef(command)
		}
	}
	if _, ok := cobraBuiltins[name]; ok {
		return commandCategory{id: categorySDTDiagnostics, title: categoryTitles[categorySDTDiagnostics]}, true
	}
	return commandCategory{}, false
}
