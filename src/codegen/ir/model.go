package ir

type ModulePlan struct {
	Packages []PackagePlan `json:"packages"`
}

type PackagePlan struct {
	ImportPath  string        `json:"importPath"`
	PackageName string        `json:"packageName"`
	Dir         string        `json:"dir"`
	Handlers    []HandlerPlan `json:"handlers"`
}

type HandlerPlan struct {
	HandlerType string     `json:"handlerType"`
	InputType   string     `json:"inputType"`
	OutputType  string     `json:"outputType"`
	Route       RoutePlan  `json:"route"`
	Input       InputPlan  `json:"input"`
	Output      OutputPlan `json:"output"`
}

type RoutePlan struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type InputPlan struct {
	Body   *InputFieldPlan  `json:"body,omitempty"`
	Fields []InputFieldPlan `json:"fields"`
}

type InputFieldPlan struct {
	GoName      string `json:"goName"`
	Source      string `json:"source"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Schema      string `json:"schema,omitempty"`
}

type OutputPlan struct {
	SharedFields []OutputFieldPlan   `json:"sharedFields"`
	Variants     []OutputVariantPlan `json:"variants"`
}

type OutputVariantPlan struct {
	StatusCode int               `json:"statusCode"`
	Fields     []OutputFieldPlan `json:"fields"`
}

type OutputFieldPlan struct {
	GoName      string `json:"goName"`
	Source      string `json:"source"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Schema      string `json:"schema,omitempty"`
}
