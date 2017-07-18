package kong

// NodeInformationResponse Node info response object
type NodeInformationResponse struct {
	Hostname   string `json:"hostname"`
	LuaVersion string `json:"lua_version"`
	Version    string `json:"version"`
}

// UpstreamRequest Upstream request object
type UpstreamRequest struct {
	Name string `json:"name"`
}

// UpstreamResponse Upstream response object
type UpstreamResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slots     uint   `json:"slots"`
	CreatedAt uint   `json:"created_at"`
}

// TargetRequest Target request object
type TargetRequest struct {
	Target string `json:"target"`
	Weight uint   `json:"weight"`
}

// TargetResponse Target response object
type TargetResponse struct {
	ID         string `json:"id"`
	Target     string `json:"target"`
	Weight     uint   `json:"weight"`
	UpstreamID string `json:"upstream_id"`
	CreatedAt  uint   `json:"created_at"`
}

// TargetListResponse Target response object
type TargetListResponse struct {
	Total uint             `json:"total"`
	Data  []TargetResponse `json:"data"`
}

// LightTargetListResponse Target light response object
type LightTargetListResponse struct {
	Total uint `json:"total"`
}
