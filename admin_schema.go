package main

import "database/sql"

func ensureAdminSchema(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS schedule_entries (
		id BIGSERIAL PRIMARY KEY,
		assignment_id BIGINT NOT NULL
			REFERENCES teacher_assignments(id) ON DELETE CASCADE,
		day_of_week INTEGER NOT NULL
			CHECK (day_of_week BETWEEN 1 AND 7),
		start_time TIME NOT NULL,
		end_time TIME NOT NULL,
		room VARCHAR(100) NOT NULL,
		lesson_type VARCHAR(50) NOT NULL DEFAULT 'Лекция',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		CHECK (end_time > start_time)
	);

	CREATE INDEX IF NOT EXISTS idx_schedule_day
	ON schedule_entries(day_of_week);

	CREATE INDEX IF NOT EXISTS idx_schedule_assignment
	ON schedule_entries(assignment_id);

	CREATE TABLE IF NOT EXISTS academic_periods (
		id BIGSERIAL PRIMARY KEY,
		academic_year VARCHAR(20) NOT NULL,
		semester INTEGER NOT NULL CHECK (semester IN (1, 2)),
		period_type VARCHAR(20) NOT NULL
			CHECK (period_type IN ('module1', 'module2', 'exam')),
		title VARCHAR(100) NOT NULL,
		start_date DATE NOT NULL,
		end_date DATE NOT NULL,
		is_open BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		CHECK (end_date >= start_date),
		UNIQUE (academic_year, semester, period_type)
	);


    CREATE TABLE IF NOT EXISTS lesson_records (
      id BIGSERIAL PRIMARY KEY,
      assignment_id BIGINT NOT NULL REFERENCES teacher_assignments(id) ON DELETE CASCADE,
      student_id BIGINT NOT NULL REFERENCES students(user_id) ON DELETE CASCADE,
      lesson_date DATE NOT NULL,
      lesson_type VARCHAR(50) NOT NULL,
      attendance VARCHAR(10) NOT NULL CHECK (attendance IN ('present','absent','excused','sick')),
      points NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (points >= 0 AND points <= 25),
      comment TEXT NOT NULL DEFAULT '',
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      UNIQUE(assignment_id, student_id, lesson_date)
    );
    CREATE TABLE IF NOT EXISTS module_scores (
      id BIGSERIAL PRIMARY KEY,
      assignment_id BIGINT NOT NULL REFERENCES teacher_assignments(id) ON DELETE CASCADE,
      student_id BIGINT NOT NULL REFERENCES students(user_id) ON DELETE CASCADE,
      module_number INTEGER NOT NULL CHECK(module_number IN (1,2)),
      lectures NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK(lectures BETWEEN 0 AND 21),
      labs NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK(labs BETWEEN 0 AND 21),
      other_work NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK(other_work BETWEEN 0 AND 21),
      extra_points NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK(extra_points BETWEEN 0 AND 4),
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      UNIQUE(assignment_id, student_id, module_number)
    );
    CREATE TABLE IF NOT EXISTS course_materials (
      id BIGSERIAL PRIMARY KEY,
      assignment_id BIGINT NOT NULL REFERENCES teacher_assignments(id) ON DELETE CASCADE,
      title VARCHAR(200) NOT NULL,
      material_type VARCHAR(30) NOT NULL,
      file_name VARCHAR(255) NOT NULL,
      file_path TEXT NOT NULL,
      uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS group_curators (
      group_id BIGINT PRIMARY KEY REFERENCES study_groups(id) ON DELETE CASCADE,
      teacher_id BIGINT NOT NULL REFERENCES teachers(id) ON DELETE CASCADE,
      assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    -- Персональный планировщик учебных задач студента.
    CREATE TABLE IF NOT EXISTS personal_tasks (
      id BIGSERIAL PRIMARY KEY,
      user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      title VARCHAR(200) NOT NULL,
      description TEXT NOT NULL DEFAULT '',
      due_at TIMESTAMPTZ,
      priority SMALLINT NOT NULL DEFAULT 2 CHECK (priority BETWEEN 1 AND 3),
      is_completed BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_personal_tasks_user_due ON personal_tasks(user_id, due_at);

    -- Портфолио: проекты, сертификаты, навыки и лучшие работы.
    CREATE TABLE IF NOT EXISTS portfolio_items (
      id BIGSERIAL PRIMARY KEY,
      student_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      title VARCHAR(200) NOT NULL,
      description TEXT NOT NULL DEFAULT '',
      category VARCHAR(30) NOT NULL DEFAULT 'project'
        CHECK (category IN ('project','certificate','lab','achievement','skill','other')),
      skills TEXT NOT NULL DEFAULT '',
      resource_url TEXT NOT NULL DEFAULT '',
      is_public BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_portfolio_student ON portfolio_items(student_user_id, created_at DESC);

    CREATE TABLE IF NOT EXISTS course_topics (
      id BIGSERIAL PRIMARY KEY,
      assignment_id BIGINT NOT NULL REFERENCES teacher_assignments(id) ON DELETE CASCADE,
      title VARCHAR(200) NOT NULL,
      description TEXT NOT NULL DEFAULT '',
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      UNIQUE(assignment_id, title)
    );
    CREATE INDEX IF NOT EXISTS idx_course_topics_assignment ON course_topics(assignment_id, created_at);

    CREATE TABLE IF NOT EXISTS conversations (
      id BIGSERIAL PRIMARY KEY,
      conversation_type VARCHAR(20) NOT NULL DEFAULT 'direct' CHECK (conversation_type IN ('direct','group')),
      title VARCHAR(200) NOT NULL DEFAULT '',
      created_by BIGINT NOT NULL REFERENCES users(id),
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS conversation_members (
      conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
      user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      last_read_at TIMESTAMPTZ,
      PRIMARY KEY(conversation_id,user_id)
    );
    CREATE TABLE IF NOT EXISTS messages (
      id BIGSERIAL PRIMARY KEY,
      conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
      sender_id BIGINT NOT NULL REFERENCES users(id),
      body TEXT NOT NULL DEFAULT '',
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
      edited_at TIMESTAMPTZ,
      deleted_at TIMESTAMPTZ,
      CHECK (length(body) <= 5000)
    );
    CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id,created_at DESC);

    CREATE TABLE IF NOT EXISTS ai_chat_messages (
      id BIGSERIAL PRIMARY KEY,
      user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      role VARCHAR(10) NOT NULL CHECK (role IN ('user','assistant')),
      content TEXT NOT NULL,
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_ai_chat_user_created ON ai_chat_messages(user_id,created_at DESC);

    CREATE TABLE IF NOT EXISTS academic_settings (
      id SMALLINT PRIMARY KEY CHECK (id=1),
      max_absence_percent NUMERIC(5,2) CHECK (max_absence_percent IS NULL OR max_absence_percent BETWEEN 0 AND 100),
      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    INSERT INTO academic_settings(id,max_absence_percent) VALUES(1,NULL) ON CONFLICT(id) DO NOTHING;

    CREATE TABLE IF NOT EXISTS notifications (
      id BIGSERIAL PRIMARY KEY,
      user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      notification_type VARCHAR(40) NOT NULL,
      title VARCHAR(200) NOT NULL,
      body TEXT NOT NULL DEFAULT '',
      link TEXT NOT NULL DEFAULT '',
      read_at TIMESTAMPTZ,
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id,created_at DESC);

	`

	_, err := db.Exec(query)
	return err
}
