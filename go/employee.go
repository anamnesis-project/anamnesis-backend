package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"fmt"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type Employee struct {
	Id            int64
	Name          string
	Email         string
	CPF           string
	Password      string
	AccessAllowed bool
	Role          Role
}

type EmployeeInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	CPF      string `json:"cpf"`
	Password string `json:"password"`
}

type EmployeeOutput struct {
	Id            int    `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	CPF           string `json:"cpf"`
	AccessAllowed bool   `json:"accessAllowed"`
	Role          Role   `json:"role"`
}

type Role struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type RegisterRequest struct {
	EmployeeInput
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	Token string     `json:"token"`
	Employee  EmployeeOutput `json:"employee"`
}

type EmployeeLoginResponse struct {
	Token string `json:"token"`
}

// TODO make it so the validation matches the api description
func (r RegisterRequest) validate() map[string]string {
	// TODO add validation for email
	// TODO add validation for cpf

	errs := make(map[string]string)
	if len(r.Name) < 3 {
		errs["name"] = "name must be at least 3 characters long"
	}

	if len(r.Password) > 72 {
		errs["password"] = "password must not exceed 72 characters"
	}

	if len(r.Password) < 12 {
		errs["password"] = "password must be at least 12 characters long"
	}
	// TODO add better validation for weak passwords

	return errs
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return BadRequest()
	}

	q := `SELECT u.password_hash, u.employee_id FROM employee u WHERE u.email = $1`
	row := s.db.QueryRow(context.Background(), q, req.Email)

	var storedHash string
	var id int
	err = row.Scan(&storedHash, &id)
	if err != nil {
		return NewAPIError(http.StatusUnauthorized, "authentication attempt failed")
	}

	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password))
	if err != nil {
		return NewAPIError(http.StatusUnauthorized, "authentication attempt failed")
	}

	jwt, err := createJWT(id)
	if err != nil {
		return err
	}
	resp := EmployeeLoginResponse{Token: jwt}

	return writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) error {
	var req RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return BadRequest()
	}

	errs := req.validate()
	if len(errs) > 0 {
		return NewAPIError(http.StatusUnprocessableEntity, errs)
	}

	// Ensure email and cpf are not taken
	q := `SELECT 1 FROM employee e
	WHERE e.email = $1 OR e.cpf = $2 LIMIT 1`

	row := s.db.QueryRow(context.Background(), q, req.Email, req.CPF)
	err = row.Scan(nil)
	if err == nil {
		return NewAPIError(http.StatusConflict, "employee with this email or cpf already exists")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hash := string(hashBytes)
	
	q = `INSERT INTO employee(name, email, cpf, password_hash, access_allowed)
	VALUES($1, $2, $3, $4, FALSE)
	RETURNING employee_id, name, email, cpf, access_allowed`

	row = s.db.QueryRow(context.Background(), q, &req.Name, &req.Email, &req.CPF, &hash)

	var emp EmployeeOutput
	err = row.Scan(&emp.Id, &emp.Name, &emp.Email, &emp.CPF, &emp.AccessAllowed)
	if err != nil {
		return err
	}

	jwt, err := createJWT(emp.Id)
	// TODO if this fails here, the insertion should be cancelled
	if err != nil {
		return err
	}

	response := RegisterResponse{
		Token: jwt,
		Employee: emp,
	}
	
	return writeJSON(w, http.StatusOK, response)
}

// TODO add query parameter parsing
func (s *Server) handleGetEmployees(w http.ResponseWriter, r *http.Request) error {
	q := `SELECT e.employee_id, e.name, e.email, e.cpf, e.access_allowed, r.role_id, r.name
	FROM employee e JOIN employee_role r on e.role_id = r.role_id`

	output := make([]EmployeeOutput, 0)
	rows, err := s.db.Query(context.Background(), q)
	if err != nil {
		fmt.Println("db error:", err.Error())
		return InternalError()
	}
	defer rows.Close()

	for rows.Next() {
		var emp EmployeeOutput
		err := rows.Scan(&emp.Id, &emp.Name, &emp.Email, &emp.CPF, &emp.AccessAllowed, &emp.Role.Id, &emp.Role.Name)
	if err != nil {
			fmt.Println("scan error:", err.Error())
			return InternalError()
		}

		output = append(output, emp)
	}

	return writeJSON(w, http.StatusOK, output)
}

func (s *Server) handleGetEmployeeById(w http.ResponseWriter, r *http.Request) error {
	id, err := getPathId("id", r)
	if err != nil {
		return BadRequest()
	}

	q := `SELECT e.employee_id, e.name e.email, e.cpf, e.access_allowed, r.role_id, r.name
	FROM employee e JOIN employee_role r on e.role_id = r.role_id
	WHERE e.employee_id = $1`

	row := s.db.QueryRow(context.Background(), q, id)
	
	var emp EmployeeOutput
	err = row.Scan(&emp.Id, &emp.Name, &emp.Email, &emp.CPF, &emp.AccessAllowed, &emp.Role.Id, &emp.Role.Name)
	if err != nil {
		return writeJSON(w, http.StatusOK, nil)
	}

	return writeJSON(w, http.StatusOK, emp)
}
