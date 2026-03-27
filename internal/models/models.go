package models

// Config holds the CLI configuration.
type Config struct {
	APIURL string `json:"api_url"`
	APIKey string `json:"api_key"`
}

type PublicIdentity struct {
	PublicID string `json:"publicId"`
}

type APIKeyIdentity struct {
	PublicID   string   `json:"publicId"`
	KeyPrefix  string   `json:"keyPrefix"`
	OwnerType  string   `json:"ownerType"`
	Scopes     []string `json:"scopes"`
	ExpiresAt  *string  `json:"expiresAt"`
}

type WhoAmIResponse struct {
	APIKey       APIKeyIdentity  `json:"apiKey"`
	Organization PublicIdentity  `json:"organization"`
	User         *PublicIdentity `json:"user"`
}

type UnauthorizedSection struct {
	Unauthorized  bool     `json:"unauthorized"`
	RequiredScopes []string `json:"requiredScopes"`
}

type OrganizationContext struct {
	PublicID string  `json:"publicId"`
	Name     *string `json:"name"`
}

type UserContext struct {
	PublicID  string  `json:"publicId"`
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Email     *string `json:"email"`
}

type Integration struct {
	PublicID string `json:"publicId"`
	Provider string `json:"provider"`
	IsActive bool   `json:"isActive"`
}

type WorkspaceNode struct {
	PublicID   string  `json:"publicId"`
	ExternalID string  `json:"externalId"`
	Provider   string  `json:"provider"`
	Type       *string `json:"type"`
	SubType    *string `json:"subType"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
}

type WorkspaceLink struct {
	PublicID      string  `json:"publicId"`
	NodeAPublicID string  `json:"nodeAPublicId"`
	NodeBPublicID string  `json:"nodeBPublicId"`
	LabelAToB     *string `json:"labelAToB"`
	LabelBToA     *string `json:"labelBToA"`
	Notes         *string `json:"notes"`
}

type ComputedLink struct {
	SourcePublicID      string `json:"sourcePublicId"`
	TargetPublicID      string `json:"targetPublicId"`
	Reason              string `json:"reason"`
	Provider            string `json:"provider"`
	IntegrationPublicID string `json:"integrationPublicId"`
}

type Workspace struct {
	PublicID      string         `json:"publicId"`
	Name          string         `json:"name"`
	Color         string         `json:"color"`
	NodeCount     int            `json:"nodeCount"`
	Nodes         []WorkspaceNode `json:"nodes"`
	Links         []WorkspaceLink `json:"links"`
	ComputedLinks []ComputedLink  `json:"computedLinks"`
}

type MemorySection struct {
	Content      *string
	Unauthorized *UnauthorizedSection
}

type ContextMemory struct {
	Organization MemorySection
	User         MemorySection
}

type FullContext struct {
	Organization            OrganizationContext
	User                    *UserContext
	Integrations            []Integration
	IntegrationsUnauthorized *UnauthorizedSection
	Workspaces              []Workspace
	WorkspacesUnauthorized  *UnauthorizedSection
	Memory                  ContextMemory
}

type KnowledgeVersion struct {
	PublicID    string `json:"publicId"`
	SHA256Hash  string `json:"sha256Hash"`
	ByteSize    int    `json:"byteSize"`
	TokenCount  int    `json:"tokenCount"`
	ContentType string `json:"contentType"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type KnowledgeFile struct {
	PublicID  string  `json:"publicId"`
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	DeletedAt *string `json:"deletedAt"`
}

type KnowledgeDocument struct {
	File    KnowledgeFile     `json:"file"`
	Version *KnowledgeVersion `json:"version"`
	Content string            `json:"content"`
}

type KnowledgePathUpsertRequest struct {
	Content       string `json:"content"`
	ForceSnapshot bool   `json:"forceSnapshot,omitempty"`
}

type KnowledgeSearchRequest struct {
	Scope          string `json:"scope,omitempty"`
	Mode           string `json:"mode,omitempty"`
	Query          string `json:"query"`
	Limit          int    `json:"limit,omitempty"`
	CaseSensitive  bool   `json:"caseSensitive,omitempty"`
	IncludeContent bool   `json:"includeContent,omitempty"`
}

type MemorySearchRequest struct {
	Mode           string `json:"mode,omitempty"`
	Query          string `json:"query"`
	Limit          int    `json:"limit,omitempty"`
	CaseSensitive  bool   `json:"caseSensitive,omitempty"`
	IncludeContent bool   `json:"includeContent,omitempty"`
}

type SearchResultItem struct {
	SourceType         string  `json:"sourceType"`
	Scope              string  `json:"scope"`
	FilePublicID       string  `json:"filePublicId"`
	VersionPublicID    string  `json:"versionPublicId"`
	Path               string  `json:"path"`
	Name               string  `json:"name"`
	Date               *string `json:"date"`
	MatchCount         int     `json:"matchCount"`
	Snippet            string  `json:"snippet"`
	Content            string  `json:"content"`
	SourceUserPublicID *string `json:"sourceUserPublicId"`
}

type SearchResult struct {
	Truncated bool               `json:"truncated"`
	Results   []SearchResultItem `json:"results"`
}
