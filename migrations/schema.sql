-- Drop tables if they exist (for clean setup)
DROP TABLE IF EXISTS certificates CASCADE;
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
  guide_name VARCHAR(120) NOT NULL DEFAULT '',
  passing_year INT NOT NULL,
  division VARCHAR(10) NOT NULL,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Internships table
CREATE TABLE internships (
  id SERIAL PRIMARY KEY,
  student_prn VARCHAR(20) REFERENCES students(prn),
  organization VARCHAR(200) NOT NULL,
  description TEXT,
  start_date DATE NOT NULL,
  end_date DATE NOT NULL,
  mode VARCHAR(10) NOT NULL CHECK (mode IN ('online', 'offline', 'hybrid')),
  credits INT NOT NULL CHECK (credits > 0),
  monthly_stipend NUMERIC(10,2) NOT NULL CHECK (monthly_stipend >= 0),
  stipend_currency CHAR(3) NOT NULL DEFAULT 'INR',
  status VARCHAR(20) NOT NULL CHECK (status IN ('pending','approved','rejected')) DEFAULT 'pending',
  created_by INT REFERENCES users(id),
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  approved_by INT REFERENCES users(id),
  approved_at TIMESTAMP,
  review_note TEXT,
  credit_eligible BOOLEAN NOT NULL DEFAULT TRUE,
  CHECK (end_date >= start_date)
);

-- Certificates table
CREATE TABLE certificates (
  id SERIAL PRIMARY KEY,
  internship_id INT UNIQUE NOT NULL REFERENCES internships(id) ON DELETE CASCADE,
  object_key TEXT NOT NULL,
  original_filename TEXT NOT NULL,
  mime_type VARCHAR(100) CHECK (mime_type IN ('application/pdf', 'image/png', 'image/jpeg')),
  file_size BIGINT CHECK (file_size <= 10 * 1024 * 1024),
  uploaded_by INT REFERENCES users(id),
  uploaded_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_internships_student_prn ON internships(student_prn);
CREATE INDEX idx_internships_status ON internships(status);
CREATE INDEX idx_students_year_division ON students(passing_year, division);
CREATE INDEX idx_certificates_internship_id ON certificates(internship_id);
