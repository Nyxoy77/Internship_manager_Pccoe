-- Drop tables if they exist (for clean setup)
DROP TABLE IF EXISTS internships CASCADE;
DROP TABLE IF EXISTS students CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Users table for authentication
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  username VARCHAR(50) UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role VARCHAR(20) CHECK (role IN ('admin','manager')) NOT NULL,
  name VARCHAR(100) NOT NULL,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Students master registry
CREATE TABLE students (
  prn VARCHAR(20) PRIMARY KEY,
  name VARCHAR(120) NOT NULL,
  passing_year INT NOT NULL,       
  division VARCHAR(10) NOT NULL,    -- e.g. "A", "B"
  created_at TIMESTAMP DEFAULT NOW()
);

-- Internships table
CREATE TABLE internships (
  id SERIAL PRIMARY KEY,
  student_prn VARCHAR(20)
    REFERENCES students(prn),
  organization VARCHAR(200) NOT NULL,
  description TEXT,
  start_date DATE NOT NULL,
  end_date DATE NOT NULL,
  mode VARCHAR(10) NOT NULL
    CHECK (mode IN ('online', 'offline', 'hybrid')),
  credits INT NOT NULL
    CHECK (credits > 0),
  monthly_stipend NUMERIC(10,2) NOT NULL
    CHECK (monthly_stipend >= 0),
  stipend_currency CHAR(3) NOT NULL DEFAULT 'INR',
  status VARCHAR(20) NOT NULL
    CHECK (status IN ('pending','approved','rejected'))
    DEFAULT 'pending',
  created_by INT REFERENCES users(id),
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  approved_by INT REFERENCES users(id),
  approved_at TIMESTAMP,
  credit_eligible BOOLEAN NOT NULL DEFAULT TRUE
  CHECK (end_date >= start_date)
);


-- Indexes for performance
CREATE INDEX idx_internships_student_prn ON internships(student_prn);
CREATE INDEX idx_internships_status ON internships(status);
CREATE INDEX idx_students_year_division ON students(year, division);

-- Seed data for testing
INSERT INTO users (username, password_hash, role, name) VALUES
('admin', '$2a$10$YourBcryptHashHere', 'admin', 'Admin User'),
('manager1', '$2a$10$YourBcryptHashHere', 'manager', 'Faculty Manager');

INSERT INTO students (prn, name, year, division) VALUES
('21IT001', 'John Doe', '3rd Year', 'A'),
('21IT002', 'Jane Smith', '3rd Year', 'A'),
('21IT003', 'Alice Johnson', '3rd Year', 'B'),
('21IT004', 'Bob Williams', '2nd Year', 'A');
