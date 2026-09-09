package models

// GithubUser represents a GitHub user object from the API
type GithubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	HTMLURL   string `json:"html_url"`
}

// GithubRepo represents a GitHub repository
type GithubRepo struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	FullName    string     `json:"full_name"`
	Private     bool       `json:"private"`
	Description string     `json:"description"`
	HTMLURL     string     `json:"html_url"`
	CloneURL    string     `json:"clone_url"`
	Language    string     `json:"language"`
	DefaultBranch string   `json:"default_branch"`
	Owner       GithubUser `json:"owner"`
}

// GithubBranch represents a GitHub branch
type GithubBranch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
	Protected bool `json:"protected"`
}

// GithubTreeItem represents a single item in a GitHub repository tree
type GithubTreeItem struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"` // "blob" or "tree"
	SHA  string `json:"sha"`
	Size int    `json:"size,omitempty"`
	URL  string `json:"url"`
}

// GithubTreeResponse is the response from GitHub tree API
type GithubTreeResponse struct {
	SHA       string           `json:"sha"`
	URL       string           `json:"url"`
	Tree      []GithubTreeItem `json:"tree"`
	Truncated bool             `json:"truncated"`
}

// GithubFileContent represents a file from GitHub
type GithubFileContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content"`   // base64 encoded
	Encoding    string `json:"encoding"`
}

// GithubUpdateFileRequest is the request body for updating a file
type GithubUpdateFileRequest struct {
	Owner   string `json:"owner" binding:"required"`
	Repo    string `json:"repo" binding:"required"`
	Path    string `json:"path" binding:"required"`
	Message string `json:"message" binding:"required"`
	Content string `json:"content" binding:"required"` // base64 encoded
	Branch  string `json:"branch"`
	SHA     string `json:"sha"` // required when updating existing file
}

// GithubLoginResponse is returned after successful OAuth login
type GithubLoginResponse struct {
	AccessToken string     `json:"access_token"`
	User        GithubUser `json:"user"`
}

// GithubTokenExchangeResponse is the response from GitHub OAuth token exchange
type GithubTokenExchangeResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// SaveGithubTokenRequest is used to manually save a GitHub token
type SaveGithubTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Username string `json:"username"`
}

// GithubIntegration represents a stored GitHub integration (token excluded from JSON output)
type GithubIntegration struct {
	ID            string `json:"id"`
	Token         string `json:"-"`
	Username      string `json:"username"`
	Name          string `json:"name"`
	ProjectID     string `json:"project_id"`
	EnvironmentID string `json:"environment_id"`
}

// GithubCreateRepoRequest is the request body for creating a new repository
type GithubCreateRepoRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	AutoInit    bool   `json:"auto_init"`
}