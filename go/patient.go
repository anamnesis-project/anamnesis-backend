package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)
type Sex string

const (
	Male Sex = "M"
	Female Sex = "F"
)

type PatientInput struct {
	Name        string     `json:"name"`
	CPF         string     `json:"cpf"`
	Sex         *Sex       `json:"sex"`
	DateOfBirth *time.Time `json:"dateOfBirth"`
}

type PatientOutput struct {
	Id          int        `json:"id"`
	Name        string     `json:"name"`
	CPF         string     `json:"cpf"`
	Sex         *Sex       `json:"sex"`
	DateOfBirth *time.Time `json:"dateOfBirth"`
}

func (s *Server) handleGetPatients(w http.ResponseWriter, r *http.Request) error {
	q := `SELECT p.patient_id, p.name, p.cpf, p.sex, p.date_of_birth FROM patient p`

	output := make([]PatientOutput, 0)
	rows, err := s.db.Query(context.Background(), q)
	if err != nil {
		fmt.Println("db error:", err.Error())
		return InternalError()
	}
	defer rows.Close()

	for rows.Next() {
		var p PatientOutput
		err := rows.Scan(&p.Id, &p.Name, &p.CPF, &p.Sex, &p.DateOfBirth)
		if err != nil {
			fmt.Println("scan error:", err.Error())
			return InternalError()
		}

		output = append(output, p)
	}

	return writeJSON(w, http.StatusOK, output)
}

func (s *Server) handleGetPatientById(w http.ResponseWriter, r *http.Request) error {
	id, err := getPathId("id", r)
	if err != nil {
		return BadRequest()
	}

	q := `SELECT p.patient_id, p.name, p.cpf, p.sex, p.date_of_birth FROM patient p WHERE p.patient_id = $1`
	row := s.db.QueryRow(context.Background(), q, id)
	
	var p PatientOutput
	err = row.Scan(&p.Id, &p.Name, &p.CPF, &p.Sex, &p.DateOfBirth)
	if err != nil {
		return NewAPIError(http.StatusNotFound, "patient does not exist")
	}

	return writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleGetPatientReports(w http.ResponseWriter, r *http.Request) error {
	patientId, err := getPathId("id", r)
	if err != nil {
		return BadRequest()
	}

	q := `SELECT patient_id, name, cpf, sex, date_of_birth FROM patient WHERE patient_id = $1`

	var p PatientOutput
	row := s.db.QueryRow(context.Background(), q, patientId)
	err = row.Scan(&p.Id, &p.Name, &p.CPF, &p.Sex, &p.DateOfBirth)
		if err != nil {
			return NewAPIError(http.StatusNotFound, "patient does not exist")
		}

	q = `SELECT r.report_id, r.weight, r.height, r.heart_rate,
	r.systolic_pressure, r.diastolic_pressure, r.temperature,
	r.oxygen_saturation, r.interview, r.issued_at,
	r.occupation, r.medications, r.allergies, r.diseases,
	FROM report r LEFT JOIN consultation c on r.report_id = c.report_id
	WHERE r.patient_id = $1`

	rows, err := s.db.Query(context.Background(), q, p.Id)
	if err != nil {
		return err
	}

	reports := make([]ReportOutput, 0)
	for rows.Next() {
		var r ReportOutput
		var consulted bool
		r.Patient = p
		err := rows.Scan(
			&r.Id, &r.Weight, &r.Height,
			&r.HeartRate, &r.SystolicPressure, &r.DiastolicPressure,
			&r.Temperature, &r.OxygenSaturation,
			&r.Interview, &r.IssuedAt,
			&r.Occupation, &r.Medications, &r.Allergies, &r.Diseases,
			&r.Urgency, &consulted,
		)

		if err != nil {
			return err
		}

		if consulted {
			r.Consultation, err = s.getConsultation(r.Id)
		}

		reports = append(reports, r)
	}

	return writeJSON(w, http.StatusOK, reports)
}

func (s *Server) createPatient(p PatientInput) (PatientOutput, error) {

	q := `
	INSERT INTO patient(name, cpf, sex, date_of_birth)
	VALUES($1, $2, $3, $4) RETURNING patient_id, name, cpf, sex, date_of_birth
	`

	row := s.db.QueryRow(context.Background(), q, p.Name, p.CPF, p.Sex, p.DateOfBirth)

	var out PatientOutput
	err := row.Scan(&out.Id, &out.Name, &out.CPF, &out.Sex, &out.DateOfBirth)

	return out, err
}
