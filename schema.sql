
CREATE TABLE patient (
    patient_id    SERIAL PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    cpf           VARCHAR(11) NOT NULL,
    date_of_birth DATE,
    sex           CHAR(1) NOT NULL CHECK (sex IN ('M','F'))
);

CREATE TABLE report (
    report_id          SERIAL PRIMARY KEY,
    patient_id         INTEGER NOT NULL REFERENCES patient,
    weight             NUMERIC(5, 2),
    height             INTEGER,
    heart_rate         INTEGER,
    systolic_pressure  INTEGER,
    diastolic_pressure INTEGER,
    temperature        NUMERIC(3, 1),
    oxygen_saturation  INTEGER,
    interview          JSON NOT NULL,
    issued_at          TIMESTAMP NOT NULL
);

CREATE TABLE employee_role (
    role_id SERIAL PRIMARY KEY,
    name    VARCHAR(20) NOT NULL,
);

CREATE TABLE employee (
    employee_id    SERIAL PRIMARY KEY,
    role_id        INTEGER REFERENCES employee_role,
    name           VARCHAR(255) NOT NULL,
    email          VARCHAR(255) NOT NULL,
    cpf            VARCHAR(11) NOT NULL,
    password_hash  VARCHAR(60) NOT NULL,
    access_allowed BOOLEAN DEFAULT FALSE,
);

