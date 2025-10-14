package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type Role struct {
	Id            int    `json:"id"`
	Name          string `json:"name"`
	AccessAllowed bool   `json:"accessAllowed"`
}

func (s *Server) handleGetRoles(w http.ResponseWriter, r *http.Request) error {
	q := `SELECT role_id, name, access_allowed FROM employee_role`

	rows, err := s.db.Query(context.Background(), q)
	if err != nil {
		return InternalError()
	}
	defer rows.Close()

	roles := make([]Role, 0)
	for rows.Next() {
		var role Role
		err := rows.Scan(&role.Id, &role.Name, &role.AccessAllowed)
		if err != nil {
			return InternalError()
		}
		roles = append(roles, role)
	}

	return writeJSON(w, http.StatusOK, roles)
}

func (s *Server) handleGetRoleById(w http.ResponseWriter, r *http.Request) error {
	id, err := getPathId("id", r)
	if err != nil {
		return NewAPIError(http.StatusBadRequest, "missing or invalid path id")
	}

	q := `SELECT role_id, name, access_allowed FROM employee_role WHERE role_id = $1`

	row := s.db.QueryRow(context.Background(), q, id)

	var role Role
	err = row.Scan(&role.Id, &role.Name, &role.AccessAllowed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "role does not exist")
		}
		return err
	}

	return writeJSON(w, http.StatusOK, role)
}
