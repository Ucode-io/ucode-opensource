package models

type ExtractUserFromPKCS7Response struct {
	SubjectCertificateInfo struct {
		SerialNumber string `json:"serialNumber"`
		X500Name     string `json:"X500Name"`
		SubjectName  struct {
			ST      string `json:"ST"`
			SURNAME string `json:"SURNAME"`
			C       string `json:"C"`
			TIN     string `json:"1.2.860.3.16.1.2"`
			CN      string `json:"CN"`
			L       string `json:"L"`
			Name    string `json:"Name"`
		} `json:"subjectName"`
		ValidFrom string `json:"validFrom"`
		ValidTo   string `json:"validTo"`
	} `json:"subjectCertificateInfo"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}
