package monitor

// HeaderField is an HTTP request header sent by an external monitor.
//
// This mirrors the Mackerel wire format but is declared here rather than
// reused from api/v1alpha1 so that the intermediate model stays independent
// of any single CRD version.
type HeaderField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DesiredExternalMonitor struct {
	Name                            string `json:"name,omitempty"`
	Service                         string `json:"service,omitempty"`
	URL                             string `json:"url"`
	Method                          string `json:"method,omitempty"`
	NotificationInterval            *int   `json:"notificationInterval,omitempty"`
	ExpectedStatusCode              *int   `json:"expectedStatusCode,omitempty"`
	ContainsString                  string `json:"containsString,omitempty"`
	ResponseTimeDuration            *int   `json:"responseTimeDuration,omitempty"`
	ResponseTimeWarning             *int   `json:"responseTimeWarning,omitempty"`
	ResponseTimeCritical            *int   `json:"responseTimeCritical,omitempty"`
	CertificationExpirationWarning  *int   `json:"certificationExpirationWarning,omitempty"`
	CertificationExpirationCritical *int   `json:"certificationExpirationCritical,omitempty"`
	IsMute                          bool   `json:"isMute,omitempty"`
	FollowRedirect                  bool   `json:"followRedirect,omitempty"`
	SkipCertificateVerification     bool   `json:"skipCertificateVerification,omitempty"`
	MaxCheckAttempts                int    `json:"maxCheckAttempts,omitempty"`
	RequestBody                     string `json:"requestBody,omitempty"`
	// Dualstack is empty when the CR omits it. The zero value is kept out of
	// the JSON so that adding this field does not change the hash of an
	// ExternalMonitor that never sets it. The ipv4 semantics of an empty value
	// are applied downstream, in the planner comparison and the provider
	// payload, rather than being materialised here.
	Dualstack string `json:"dualstack,omitempty"`
	// Headers is nil when the CR does not declare headers, which means the
	// operator leaves them to Mackerel. A non-nil empty slice means "remove
	// every header". The pointer keeps the two apart in the desired hash,
	// where omitempty would otherwise drop both.
	Headers  *[]HeaderField `json:"headers,omitempty"`
	Memo     string         `json:"memo,omitempty"`
	Resource string         `json:"resource"`
	Owner    string         `json:"owner"`
	Hash     string         `json:"hash,omitempty"`
}

type ActualExternalMonitor struct {
	ID                              string
	Name                            string
	Service                         string
	URL                             string
	Method                          string
	NotificationInterval            *int
	ExpectedStatusCode              *int
	ContainsString                  string
	ResponseTimeDuration            *int
	ResponseTimeWarning             *int
	ResponseTimeCritical            *int
	CertificationExpirationWarning  *int
	CertificationExpirationCritical *int
	IsMute                          bool
	FollowRedirect                  bool
	SkipCertificateVerification     bool
	MaxCheckAttempts                int
	RequestBody                     string
	Dualstack                       string
	Headers                         []HeaderField
	Memo                            string
}
