package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"

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
	Role          Role   `json:"role"`
}

type RegisterResponse struct {
	Token string     `json:"token"`
	Employee  EmployeeOutput `json:"employee"`
}

type EmployeeLoginResponse struct {
	Token string `json:"token"`
	Employee  EmployeeOutput `json:"employee"`
}

type RegisterRequest struct {
	EmployeeInput
}

func (r RegisterRequest) validate() map[string][]string {
	errs := make(map[string][]string)

	if len(r.Name) < 3 {
		errs["name"] = append(errs["name"], "name must be at least 3 characters long")
	}

	if len(r.Password) > 72 {
		errs["password"] = append(errs["password"], "password must not exceed 72 characters")
	}

	if len(r.Password) < 12 {
		errs["password"] = append(errs["password"], "password must be at least 12 characters long")
	}

	if !ContainsNumber(r.Password) {
		errs["password"] = append(errs["password"], "password must contain a number")
	}

	if !ContainsLowerCaseLetter(r.Password) {
		errs["password"] = append(errs["password"], "password must contain a lower case letter")
	}

	if !ContainsUpperCaseLetter(r.Password) {
		errs["password"] = append(errs["password"], "password must contain an upper case letter")
	}

	if !ContainsSpecialCharacter(r.Password) {
		errs["password"] = append(errs["password"], "password must contain at least 1 special character")
	}

	_, err := mail.ParseAddress(r.Email)
	if err != nil {
		errs["email"] = append(errs["email"], "email is invalid")
	}

	if !ValidateCPF(r.CPF) {
		errs["cpf"] = append(errs["cpf"], "invalid CPF")
	}

	return errs
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r LoginRequest) validate() map[string][]string {
	errs := make(map[string][]string)

	if len(r.Password) > 72 {
		errs["password"] = append(errs["password"], "password must not exceed 72 characters")
	}

	if len(r.Password) < 12 {
		errs["password"] = append(errs["password"], "password must be at least 12 characters long")
	}

	if !ContainsNumber(r.Password) {
		errs["password"] = append(errs["password"], "password must contain a number")
	}

	if !ContainsLowerCaseLetter(r.Password) {
		errs["password"] = append(errs["password"], "password must contain a lower case letter")
	}

	if !ContainsUpperCaseLetter(r.Password) {
		errs["password"] = append(errs["password"], "password must contain an upper case letter")
	}

	if !ContainsSpecialCharacter(r.Password) {
		errs["password"] = append(errs["password"], "password must contain at least 1 special character")
	}

	_, err := mail.ParseAddress(r.Email)
	if err != nil {
		errs["email"] = append(errs["email"], "email is invalid")
	}

	return errs
}

type PatchEmployeeRequest struct {
	RoleId int `json:"roleId"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return BadRequest()
	}

	errs := req.validate()
	if len(errs) > 0 {
		return NewAPIError(http.StatusUnprocessableEntity, errs)
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
	// NOTE this query could be removed by fetching everything together with the password hash, but I don't care :)
	resp.Employee, err = s.getEmployee(id)
	if err != nil {
		return NewAPIError(http.StatusUnauthorized, "authentication attempt failed")
	}

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
	
	q = `
	INSERT INTO employee(name, email, cpf, password_hash)
	VALUES($1, $2, $3, $4) RETURNING employee_id
	`

	row = s.db.QueryRow(context.Background(), q, req.Name, req.Email, req.CPF, hash)

	var newEntryId int
	err = row.Scan(&newEntryId)
	if err != nil {
		return err
	}

	emp, err := s.getEmployee(newEntryId)
	if err != nil {
		return err
	}

	jwt, err := createJWT(emp.Id)
	if err != nil {
		return err
	}

	response := RegisterResponse{
		Token: jwt,
		Employee: emp,
	}
	
	return writeJSON(w, http.StatusCreated, response)
}

func (s *Server) handleGetEmployees(w http.ResponseWriter, r *http.Request) error {
	q := `SELECT e.employee_id, e.name, e.email, e.cpf, r.role_id, r.name, r.access_allowed
	FROM employee e JOIN employee_role r on e.role_id = r.role_id`

	queryParams := r.URL.Query()
	accessAllowedFilter, err := strconv.ParseBool(queryParams.Get("accessAllowed"))

	var rows pgx.Rows
	// err means there is no filter applied
	if err != nil {
		rows, err = s.db.Query(context.Background(), q)
		if err != nil {
			fmt.Println("db error:", err.Error())
			return InternalError()
		}
	} else {
		q += " AND r.access_allowed = $1"
		rows, err = s.db.Query(context.Background(), q, accessAllowedFilter)
		if err != nil {
			fmt.Println("db error:", err.Error())
			return InternalError()
		}
	}
	output := make([]EmployeeOutput, 0)
	defer rows.Close()

	for rows.Next() {
		var emp EmployeeOutput
		err := rows.Scan(&emp.Id, &emp.Name, &emp.Email, &emp.CPF, &emp.Role.Id, &emp.Role.Name, &emp.Role.AccessAllowed)
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

	emp, err := s.getEmployee(id)
	if err != nil {
		return NewAPIError(http.StatusNotFound, "employee does not exist")
	}

	return writeJSON(w, http.StatusOK, emp)
}

func (s *Server) handlePatchEmployeePermissions(w http.ResponseWriter, r *http.Request) error {
	employeeId, err := getPathId("id", r)
	if err != nil {
		return BadRequest()
	}

	var req PatchEmployeeRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return BadRequest()
	}

	q := `SELECT 1 FROM employee_role WHERE role_id = $1 LIMIT 1`

	row := s.db.QueryRow(context.Background(), q, req.RoleId)
	if err = row.Scan(nil); err != nil {
		return NewAPIError(http.StatusBadRequest, "selected role does not exist")
	}

	q = `UPDATE employee SET role_id = $1 WHERE employee_id = $2`

	_, err = s.db.Exec(context.Background(), q, req.RoleId, employeeId)
	if err != nil {
		return err
	}
	
	emp, err := s.getEmployee(employeeId)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, emp)
}

func (s *Server) getEmployeeAccess(employeeId int) (bool, error) {
	q := `
	SELECT r.access_allowed FROM employee e
	JOIN employee_role r ON e.role_id = r.role_id
	WHERE e.employee_id = $1
	`
	row := s.db.QueryRow(context.Background(), q, employeeId)

	var accessAllowed bool
	err := row.Scan(&accessAllowed)
	if err != nil {
		return false, err
	}

	return accessAllowed, nil
}

func (s *Server) getEmployee(id int) (EmployeeOutput, error) {
	q := `SELECT e.employee_id, e.name, e.email, e.cpf, r.role_id, r.name, r.access_allowed
	FROM employee e JOIN employee_role r on e.role_id = r.role_id
	WHERE e.employee_id = $1`

	row := s.db.QueryRow(context.Background(), q, id)

	var emp EmployeeOutput
	err := row.Scan(&emp.Id, &emp.Name, &emp.Email, &emp.CPF, &emp.Role.Id, &emp.Role.Name, &emp.Role.AccessAllowed)
	if err != nil {
		fmt.Println(err)
	}

	return emp, err
}
