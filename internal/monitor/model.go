package monitor

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
	Memo                            string `json:"memo,omitempty"`
	Resource                        string `json:"resource"`
	Owner                           string `json:"owner"`
	Hash                            string `json:"hash,omitempty"`
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
	Memo                            string
}
