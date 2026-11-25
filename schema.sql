
-- MY EDU: основная структура базы данных

-- 1. Пользователи
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    full_name VARCHAR(150) NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(20) NOT NULL
        CHECK (role IN ('admin', 'teacher', 'student')),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. Факультеты
CREATE TABLE IF NOT EXISTS faculties (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 3. Кафедры
CREATE TABLE IF NOT EXISTS departments (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    faculty_id BIGINT NOT NULL REFERENCES faculties(id),
    UNIQUE(name, faculty_id)
);

-- 4. Учебные группы
CREATE TABLE IF NOT EXISTS study_groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    department_id BIGINT NOT NULL REFERENCES departments(id),
    admission_year INTEGER NOT NULL,
    course_number INTEGER NOT NULL CHECK (course_number BETWEEN 1 AND 6)
);

-- 5. Студенты
CREATE TABLE IF NOT EXISTS students (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id),
    group_id BIGINT NOT NULL REFERENCES study_groups(id),
    student_number VARCHAR(50) UNIQUE NOT NULL,
    admission_year INTEGER NOT NULL
);

-- 6. Преподаватели
CREATE TABLE IF NOT EXISTS teachers (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id),
    department_id BIGINT NOT NULL REFERENCES departments(id),
    employee_number VARCHAR(50) UNIQUE NOT NULL,
    position VARCHAR(100)
);

-- 7. Дисциплины
CREATE TABLE IF NOT EXISTS subjects (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    subject_code VARCHAR(50) UNIQUE,
    department_id BIGINT NOT NULL REFERENCES departments(id)
);

-- 8. Назначение преподавателей на предметы и группы
CREATE TABLE IF NOT EXISTS teacher_assignments (
    id BIGSERIAL PRIMARY KEY,
    teacher_id BIGINT NOT NULL REFERENCES teachers(id),
    subject_id BIGINT NOT NULL REFERENCES subjects(id),
    group_id BIGINT NOT NULL REFERENCES study_groups(id),
    academic_year VARCHAR(20) NOT NULL,
    semester INTEGER NOT NULL CHECK (semester IN (1, 2)),
    UNIQUE(teacher_id, subject_id, group_id, academic_year, semester)
);

-- Индексы для быстрого поиска
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_students_group ON students(group_id);
CREATE INDEX IF NOT EXISTS idx_teachers_department ON teachers(department_id);
CREATE INDEX IF NOT EXISTS idx_assignments_teacher ON teacher_assignments(teacher_id);
CREATE INDEX IF NOT EXISTS idx_assignments_group ON teacher_assignments(group_id);
