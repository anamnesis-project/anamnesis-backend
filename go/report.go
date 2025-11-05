package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/signintech/gopdf"
)

type Urgency string

const (
	Undefined Urgency = "undefined"
	Green     Urgency = "green"
	Yellow    Urgency = "Yellow"
	Red       Urgency = "red"
)

type ReportBase struct {
	Weight             float32 `json:"weight"`
	Height             int     `json:"height"`
	HeartRate         *int     `json:"heartRate"`
	SystolicPressure  *int     `json:"systolicPressure"`
	DiastolicPressure *int     `json:"diastolicPressure"`
	Temperature       *float32 `json:"temperature"`
	OxygenSaturation  *int     `json:"oxygenSaturation"`
	Interview         []QA     `json:"interview"`
	Occupation        string   `json:"occupation"`
	Medications       []string  `json:"medications"`
	Allergies         []string  `json:"allergies"`
	Diseases          []string  `json:"diseases"`
}

type ReportOutput struct {
	Id           int           `json:"id"`
	Patient      PatientOutput `json:"patient"`
	ReportBase
	IssuedAt     time.Time     `json:"issuedAt"`
	Urgency      Urgency       `json:"urgency"`
	Consultation *Consultation `json:"consultation,omitempty"`
}

type QA struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type Consultation struct {
	DoctorId         int       `json:"doctorId,omitempty"`
	// using time.Time as pointer is a workaround to make sure no json parsing when zero value is given
	ConsultationDate *time.Time `json:"consultationDate,omitempty"`
}

type CreateReportRequest struct {
	ReportBase
	Patient PatientInput `json:"patient"`
}

func (r CreateReportRequest) validate() map[string][]string {
	errs := make(map[string][]string)

	if len(r.Patient.Name) == 0 {
		errs["name"] = append(errs["name"], "patient name missing")
	}

	if !ValidateCPF(r.Patient.CPF) {
		errs["cpf"] = append(errs["cpf"], "invalid CPF")
	}

	if r.Patient.Sex != Male && r.Patient.Sex != Female {
		errs["sex"] = append(errs["sex"], "invalid sex")
	}

	if !r.Patient.DateOfBirth.Before(time.Now()) {
		errs["dateOfBirth"] = append(errs["dateOfBirth"], "invalid date of birth")
	}

	if r.Weight < 0 {
		errs["weight"] = append(errs["weight"], "weight must be greater than 0 Kg")
	}

	if r.Height < 0 {
		errs["height"] = append(errs["height"], "height must be greater than 0 cm")
	}

	if r.HeartRate != nil && *r.HeartRate < 0 {
		errs["heartRate"] = append(errs["heartRate"], "heart rate must be greater than 0 bpm")
	}

	if r.SystolicPressure != nil && *r.SystolicPressure < 0 {
		errs["systolicPressure"] = append(errs["systolicPressure"], "systolic pressure must be greater than 0")
	}

	if r.DiastolicPressure != nil && *r.DiastolicPressure < 0 {
		errs["diastolicPressure"] = append(errs["diastolicPressure"], "diastolic pressure must be greater than 0")
	}

	if r.Temperature != nil && *r.Temperature < 0 {
		errs["temperature"] = append(errs["temperature"], "temperature must be greater than 0 C")
	}

	if r.OxygenSaturation != nil {
		if *r.OxygenSaturation < 0 {
			errs["saturation"] = append(errs["saturation"], "saturation must be greater than 0%")
		}

		if *r.OxygenSaturation > 100 {
			errs["saturation"] = append(errs["saturation"], "saturation must be at most 100%")
		}
	}

	if len(r.Interview) == 0 {
		errs["interview"] = append(errs["interview"], "interview must not be empty")
	}

	return errs
}

type ChangeUrgencyRequest struct {
	Urgency Urgency `json:"urgency"`
}

func (r ChangeUrgencyRequest) validate() map[string][]string {
	errs := make(map[string][]string)
	if r.Urgency != Undefined &&
	r.Urgency != Green &&
	r.Urgency != Yellow &&
	r.Urgency != Red {
		errs["urgency"] = append(errs["urgency"], "invalid urgency type")
	}

	return errs
}

func (s *Server) handleGetReports(w http.ResponseWriter, r *http.Request) error {
	q := `SELECT r.report_id, r.weight, r.height, r.heart_rate,
	r.systolic_pressure, r.diastolic_pressure, r.temperature,
	r.oxygen_saturation, r.interview, r.issued_at,
	r.occupation, r.medications, r.allergies, r.diseases,
	p.patient_id, p.name, p.cpf, p.sex, p.date_of_birth,
	r.urgency, (c.report_id IS NOT NULL) AS consulted
	FROM report r JOIN patient p on r.patient_id = p.patient_id
	LEFT JOIN consultation c on r.report_id = c.report_id
	`

	output := make([]ReportOutput, 0)
	rows, err := s.db.Query(context.Background(), q)
	if err != nil {
		fmt.Println("db error:", err.Error())
		return InternalError()
	}
	defer rows.Close()

	for rows.Next() {
		var r ReportOutput
		var consulted bool
		err := rows.Scan(
			&r.Id, &r.Weight, &r.Height,
			&r.HeartRate, &r.SystolicPressure, &r.DiastolicPressure,
			&r.Temperature, &r.OxygenSaturation,
			&r.Interview, &r.IssuedAt,
			&r.Occupation, &r.Medications, &r.Allergies, &r.Diseases,
			&r.Patient.Id, &r.Patient.Name, &r.Patient.CPF,
			&r.Patient.Sex, &r.Patient.DateOfBirth,
			&r.Urgency, &consulted)

		if err != nil {
			fmt.Println("scan error:", err.Error())
			return InternalError()
		}

		if consulted {
			r.Consultation, err = s.getConsultation(r.Id)
		}

		output = append(output, r)
	}

	return writeJSON(w, http.StatusOK, output)
}

func (s *Server) handleGetReportById(w http.ResponseWriter, r *http.Request) error {
	id, err := getPathId("id", r)
	if err != nil {
		return BadRequest()
	}

	rep, err := s.getReportById(id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "report does not exist")
		}

		return err
	}

	return writeJSON(w, http.StatusOK, rep)
}

// TODO add new fields to report
func (s *Server) handleGetReportPDF(w http.ResponseWriter, r *http.Request) error {
	id, err := getPathId("id", r)
	if err != nil {
		return BadRequest()
	}

	rep, err := s.getReportById(id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "report does not exist")
		}

		return err
	}

	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{ PageSize: *gopdf.PageSizeA4 })
	pdf.AddPage()
	pdf.SetMarginLeft(100)
	pdf.SetMarginRight(100)
	err = pdf.AddTTFFont("arial", "../fonts/arial/ARIAL.TTF")
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = pdf.SetFont("arial", "", 14)
	if err != nil {
		fmt.Println(err)
		return err
	}

	pdf.SetFontSize(24)
	pdf.SetXY(pdf.MarginLeft(), 50)
	pdf.Text(rep.Patient.Name)
	// W: 595, H: 842
	pdf.SetFontSize(12)
	pdf.SetXY(pdf.MarginLeft(), 60)
	rect := gopdf.Rect{ W: 100, H: 32 }
	pdf.Cell(&rect, fmt.Sprintf("Sex: %s", rep.Patient.Sex))
	pdf.Cell(&rect, fmt.Sprintf("Date of Birth: %s", rep.Patient.DateOfBirth.Format("02/01/2006")))

	pdf.SetXY(400, 70)
	pdf.Text(fmt.Sprintf("Issued at: %s", rep.IssuedAt.Format("02/01/2006")))

	y := 100.0

	pdf.SetXY(pdf.MarginLeft(), y)
	pdf.Text(fmt.Sprintf("Urgency: %s", rep.Urgency))
	y += 20

	pdf.SetXY(pdf.MarginLeft(), y)
	pdf.Text(fmt.Sprintf("Height: %.2f m", float32(rep.Height / 100)))
	y += 20

	pdf.SetXY(pdf.MarginLeft(), y)
	pdf.Text(fmt.Sprintf("Weight: %.1f Kg", rep.Weight))
	y += 20

	pdf.SetXY(pdf.MarginLeft(), y)
	if rep.HeartRate != nil {
		pdf.Text(fmt.Sprintf("Heart Rate: %d BPM", *rep.HeartRate))
	} else {
		pdf.Text("Heart Rate: N/A")
	}
	y += 20

	pdf.SetXY(pdf.MarginLeft(), y)
	if rep.OxygenSaturation != nil {
		pdf.Text(fmt.Sprintf("Oxygen Saturation: %d%%", *rep.OxygenSaturation))
	} else {
		pdf.Text("Oxygen Saturation: N/A")
	}
	y += 20

	pdf.SetXY(pdf.MarginLeft(), y)
	if rep.Temperature != nil {
		pdf.Text(fmt.Sprintf("Temperature: %.1f °C", *rep.Temperature))
	} else {
		pdf.Text("Temperature: N/A")
	}
	y += 20

	pdf.SetXY(pdf.MarginLeft(), y)
	if rep.SystolicPressure != nil && rep.DiastolicPressure != nil {
		pdf.Text(fmt.Sprintf("Blood pressure: %d/%d", *rep.SystolicPressure, *rep.DiastolicPressure))
	} else {
		pdf.Text("Blood pressure: N/A")
	}
	y += 36

	pdf.SetXY(pdf.MarginLeft(), y)
	pdf.SetFontSize(20)
	pdf.Text("Interview")
	y += 16
	pdf.SetFontSize(12)

	for _, qa := range rep.Interview {
		pdf.SetXY(pdf.MarginLeft(), y)
		pdf.Text(fmt.Sprintf("Question: %s", qa.Question))
		y += 20

		pdf.SetXY(pdf.MarginLeft(), y)
		pdf.Text(fmt.Sprintf("Answer: %s", qa.Answer))
		y += 32
	}

	return writePDF(w, &pdf)
}

func (s *Server) handleChangeReportUrgency(w http.ResponseWriter, r *http.Request) error {
	employeeId, err := getIdFromToken(r)
	if err != nil {
		return InvalidToken()
	}

	validPermissions, err := s.getEmployeeAccess(employeeId)
	if err != nil {
		return InternalError()
	}

	if !validPermissions {
		return AccessNotAllowed()
	}

	reportId, err := getPathId("id", r)
	if err != nil {
		return BadRequest()
	}

	var req ChangeUrgencyRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return BadRequest()
	}

	errs := req.validate()
	if len(errs) > 0 {
		return NewAPIError(http.StatusUnprocessableEntity, errs)
	}

	q := `UPDATE report SET urgency = $1 WHERE report_id = $2`
	_, err = s.db.Exec(context.Background(), q, req.Urgency, reportId)
	if err != nil {
		return err
	}

	rep, err := s.getReportById(reportId)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleCreateConsultation(w http.ResponseWriter, r *http.Request) error {
	employeeId, err := getIdFromToken(r)
	if err != nil {
		return InvalidToken()
	}

	validPermissions, err := s.getEmployeeAccess(employeeId)
	if err != nil {
		return InternalError()
	}

	if !validPermissions {
		return AccessNotAllowed()
	}

	reportId, err := getPathId("id", r)
	if err != nil {
		return BadRequest()
	}

	rep, err := s.getReportById(reportId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NewAPIError(http.StatusBadRequest, "report does not exist")
		}
	}

	now := time.Now()
	q := `INSERT INTO consultation(report_id, doctor_id, consultation_date) VALUES($1, $2, $3)`
	_, err = s.db.Exec(context.Background(), q, reportId, employeeId, now)
	if err != nil {
		return err
	}

	rep.Consultation = &Consultation{
		DoctorId: employeeId,
		ConsultationDate: &now,
	}

	return writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) error {
	var req CreateReportRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		fmt.Println(err)
		return BadRequest()
	}

	errs := req.validate()
	if len(errs) > 0 {
		return NewAPIError(http.StatusUnprocessableEntity, errs)
	}

	q := `SELECT p.patient_id, p.name, p.cpf, p.sex, p.date_of_birth
	FROM patient p WHERE p.cpf = $1 LIMIT 1`

	row := s.db.QueryRow(context.Background(), q, req.Patient.CPF)
	var patient PatientOutput
	err = row.Scan(
		&patient.Id, &patient.Name, &patient.CPF,
		&patient.Sex, &patient.DateOfBirth)

	if err != nil {
		// patient doesnt exist 
		patient, err = s.createPatient(req.Patient)
		if err != nil {
			return err
		}
	}

	q = `
	INSERT INTO report(patient_id, weight, height, heart_rate, systolic_pressure,
	diastolic_pressure, temperature, oxygen_saturation, interview, issued_at,
	occupation, medications, allergies, diseases)
	VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	RETURNING report_id, weight, height, heart_rate, systolic_pressure,
	diastolic_pressure, temperature, oxygen_saturation, interview, issued_at,
	occupation, medications, allergies, diseases
	`

	row = s.db.QueryRow(context.Background(), q,
		patient.Id, req.Weight, req.Height, req.HeartRate,
		req.SystolicPressure, req.DiastolicPressure, req.Temperature,
		req.OxygenSaturation, req.Interview, time.Now(),
		req.Occupation, req.Medications, req.Allergies, req.Diseases)

	var rep ReportOutput
	err = row.Scan(&rep.Id, &rep.Weight, &rep.Height, &rep.HeartRate,
		&rep.SystolicPressure, &rep.DiastolicPressure, &rep.Temperature,
		&rep.OxygenSaturation, &rep.Interview, &rep.IssuedAt,
		&rep.Occupation, &rep.Medications, &rep.Allergies, &rep.Diseases)

	if err != nil {
		return err
	}

	rep.Patient = patient
	return writeJSON(w, http.StatusCreated, rep)
}

func (s *Server) getReportById(id int) (ReportOutput, error) {
	q := `
	SELECT r.report_id, r.weight, r.height, r.heart_rate,
	r.systolic_pressure, r.diastolic_pressure, r.temperature,
	r.oxygen_saturation, r.interview, r.issued_at,
	r.occupation, r.medications, r.allergies, r.diseases,
	p.patient_id, p.name, p.cpf, p.sex, p.date_of_birth,
	r.urgency, (c.report_id IS NOT NULL) AS consulted
	FROM report r JOIN patient p on r.patient_id = p.patient_id
	LEFT JOIN consultation c on r.report_id = c.report_id
	WHERE r.report_id = $1
	`

	row := s.db.QueryRow(context.Background(), q, id)
	var rep ReportOutput
	var consulted bool
	err := row.Scan(&rep.Id, &rep.Weight, &rep.Height, &rep.HeartRate, &rep.SystolicPressure, &rep.DiastolicPressure,
		&rep.Temperature, &rep.OxygenSaturation, &rep.Interview, &rep.IssuedAt,
		&rep.Occupation, &rep.Medications, &rep.Allergies, &rep.Diseases,
		&rep.Patient.Id, &rep.Patient.Name, &rep.Patient.CPF, &rep.Patient.Sex, &rep.Patient.DateOfBirth,
		&rep.Urgency, &consulted)

	if consulted {
		rep.Consultation, err = s.getConsultation(id)
	}

	return rep, err
}

func (s *Server) getConsultation(reportId int) (*Consultation, error) {
	var c Consultation
	q := `SELECT c.doctor_id, c.consultation_date FROM consultation c WHERE report_id = $1`
	row := s.db.QueryRow(context.Background(), q, reportId)
	err := row.Scan(&c.DoctorId, &c.ConsultationDate)

	return &c, err
}
