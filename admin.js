
document.addEventListener("DOMContentLoaded", () => {
    const API = "/api/admin";

    const modal = document.getElementById("admin-modal");
    const form = document.getElementById("admin-form");
    const fields = document.getElementById("modal-fields");
    const formError = document.getElementById("form-error");
    const toast = document.getElementById("toast");

    let currentType = "";
    let currentId = null;

    let usersCache = [];
    let facultiesCache = [];
    let departmentsCache = [];
    let groupsCache = [];
    let subjectsCache = [];
    let assignmentsCache = [];
    let scheduleCache = [];
    let periodsCache = [];

    let optionsCache = {
        teachers: [],
        subjects: [],
        groups: []
    };

    const titles = {
        dashboard: "Панель администратора",
        users: "Пользователи",
        students: "Студенты",
        teachers: "Преподаватели",
        faculties: "Факультеты",
        departments: "Кафедры",
        groups: "Учебные группы",
        subjects: "Предметы",
        assignments: "Назначения преподавателей",
        schedule: "Расписание",
        "academic-periods": "Модули и экзамены",
        settings: "Настройки"
    };

    // =====================================
    // API
    // =====================================

    async function request(url, options = {}) {
        const response = await fetch(url, {
            credentials: "same-origin",
            ...options,
            headers: {
                ...(options.body
                    ? { "Content-Type": "application/json" }
                    : {}),
                ...options.headers
            }
        });

        const responseText = await response.text();
        let data = {};

        try {
            data = responseText ? JSON.parse(responseText) : {};
        } catch {
            data = { error: responseText };
        }

        if (!response.ok) {
            throw new Error(
                data.error || data.message || "Ошибка сервера"
            );
        }

        return data;
    }

    // =====================================
    // Вспомогательные функции
    // =====================================

    function notify(message) {
        if (!toast) {
            alert(message);
            return;
        }

        toast.textContent = message;
        toast.classList.add("show");

        setTimeout(() => {
            toast.classList.remove("show");
        }, 3000);
    }

    function showError(message = "") {
        if (formError) {
            formError.textContent = message;
        }
    }

    function escapeHTML(value) {
        return String(value ?? "").replace(/[&<>"']/g, char => {
            const map = {
                "&": "&amp;",
                "<": "&lt;",
                ">": "&gt;",
                '"': "&quot;",
                "'": "&#039;"
            };

            return map[char];
        });
    }

    function cell(value) {
        return `<td>${escapeHTML(value)}</td>`;
    }

    function setText(id, value) {
        const element = document.getElementById(id);

        if (element) {
            element.textContent = value;
        }
    }

    function fillTable(id, rows, emptyText = "Нет данных") {
        const table = document.getElementById(id);
        if (!table) return;

        const body = table.querySelector("tbody");
        if (!body) return;

        body.innerHTML = rows.length
            ? rows.join("")
            : `
                <tr>
                    <td colspan="10" class="empty-row">
                        ${escapeHTML(emptyText)}
                    </td>
                </tr>
            `;
    }

    function actionButtons(id, type, allowEdit = true) {
        return `
            <td>
                ${allowEdit ? `
                    <button
                        class="table-action edit-btn"
                        data-id="${id}"
                        data-type="${type}">
                        Изменить
                    </button>
                ` : ""}

                <button
                    class="table-action delete delete-btn"
                    data-id="${id}"
                    data-type="${type}">
                    Удалить
                </button>
            </td>
        `;
    }

    function roleName(role) {
        const roles = {
            admin: "Администратор",
            teacher: "Преподаватель",
            student: "Студент"
        };

        return roles[role] || role;
    }

    // =====================================
    // Переключение разделов
    // =====================================

    function showSection(name) {
        document.querySelectorAll(".admin-section").forEach(section => {
            section.classList.remove("active");
        });

        const section = document.getElementById(`section-${name}`);

        if (section) {
            section.classList.add("active");
        }

        document.querySelectorAll("[data-section]").forEach(link => {
            link.classList.toggle(
                "active",
                link.dataset.section === name
            );
        });

        setText("page-title", titles[name] || name);

        loadSection(name);
    }

    document.querySelectorAll("[data-section]").forEach(link => {
        link.addEventListener("click", event => {
            event.preventDefault();
            showSection(link.dataset.section);
        });
    });

    async function loadSection(name) {
        try {
            switch (name) {
                case "dashboard":
                    await loadDashboard();
                    break;

                case "users":
                    await loadUsers();
                    break;

                case "students":
                    await loadUsers("student");
                    break;

                case "teachers":
                    await loadUsers("teacher");
                    break;

                case "faculties":
                    await loadFaculties();
                    break;

                case "departments":
                    await loadDepartments();
                    break;

                case "groups":
                    await loadGroups();
                    break;

                case "subjects":
                    await loadSubjects();
                    break;

                case "assignments":
                    await loadAssignments();
                    break;

                case "schedule":
                    await loadSchedule();
                    break;

                case "academic-periods":
                    await loadPeriods();
                    break;
                case "settings":
                    await loadSettings();
                    break;
            }
        } catch (error) {
            notify(error.message);
        }
    }

    // =====================================
    // Главная панель
    // =====================================

    async function loadDashboard() {
        const data = await request(`${API}/dashboard`);

        setText("stat-students", data.students ?? 0);
        setText("stat-teachers", data.teachers ?? 0);
        setText("stat-groups", data.study_groups ?? data.groups ?? 0);
        setText("stat-subjects", data.subjects ?? 0);
    }

    // =====================================
    // Пользователи
    // =====================================

    async function loadUsers(role = "") {
        usersCache = await request(`${API}/users`);

        let users = usersCache;

        if (role) {
            users = users.filter(user => user.role === role);
        }

        const tableId = role === "student"
            ? "students-table"
            : role === "teacher"
                ? "teachers-table"
                : "users-table";

        const rows = users.map(user => `
            <tr>
                ${cell(user.id)}
                ${cell(user.full_name)}
                ${cell(user.email)}
                ${cell(roleName(user.role))}
                ${cell(user.is_active ? "Активен" : "Заблокирован")}
                ${actionButtons(user.id, "users")}
            </tr>
        `);

        fillTable(tableId, rows);
    }

    // =====================================
    // Факультеты
    // =====================================

    async function loadFaculties() {
        facultiesCache = await request(`${API}/faculties`);

        const rows = facultiesCache.map(item => `
            <tr>
                ${cell(item.id)}
                ${cell(item.name)}
                ${actionButtons(item.id, "faculties")}
            </tr>
        `);

        fillTable("faculties-table", rows);
    }

    // =====================================
    // Кафедры
    // =====================================

    async function loadDepartments() {
        departmentsCache = await request(`${API}/departments`);

        const rows = departmentsCache.map(item => `
            <tr>
                ${cell(item.id)}
                ${cell(item.name)}
                ${cell(item.faculty || item.faculty_name || item.faculty_id)}
                ${actionButtons(item.id, "departments")}
            </tr>
        `);

        fillTable("departments-table", rows);
    }

    // =====================================
    // Учебные группы
    // =====================================

    async function loadGroups() {
        groupsCache = await request(`${API}/groups`);

        const rows = groupsCache.map(item => `
            <tr>
                ${cell(item.id)}
                ${cell(item.name)}
                ${cell(item.department || item.department_name || item.department_id)}
                ${cell(item.admission_year)}
                ${cell(item.course_number)}
                ${actionButtons(item.id, "groups")}
            </tr>
        `);

        fillTable("groups-table", rows);
    }

    // =====================================
    // Предметы
    // =====================================

    async function loadSubjects() {
        subjectsCache = await request(`${API}/subjects`);

        const rows = subjectsCache.map(item => `
            <tr>
                ${cell(item.id)}
                ${cell(item.name)}
                ${cell(item.subject_code || "")}
                ${cell(item.department || item.department_name || item.department_id)}
                ${actionButtons(item.id, "subjects")}
            </tr>
        `);

        fillTable("subjects-table", rows);
    }

    // =====================================
    // Назначения преподавателей
    // =====================================

    async function loadAssignments() {
        assignmentsCache = await request(`${API}/assignments`);

        const rows = assignmentsCache.map(item => `
            <tr>
                ${cell(item.id)}
                ${cell(item.teacher || item.teacher_name || item.teacher_id)}
                ${cell(item.subject || item.subject_name || item.subject_id)}
                ${cell(item.group || item.group_name || item.group_id)}
                ${cell(item.academic_year)}
                ${cell(item.semester)}
                ${actionButtons(item.id, "assignments", false)}
            </tr>
        `);

        fillTable("assignments-table", rows);
    }

    // =====================================
    // Расписание
    // =====================================

    const dayNames = {
        1: "Понедельник",
        2: "Вторник",
        3: "Среда",
        4: "Четверг",
        5: "Пятница",
        6: "Суббота",
        7: "Воскресенье"
    };

    async function loadSchedule() {
        scheduleCache = await request(`${API}/schedule`);

        const container = document.getElementById("schedule-content");
        if (!container) return;

        let html = `
            <div class="section-toolbar">
                <div>
                    <h3>Расписание занятий</h3>
                    <p>Управление занятиями университета</p>
                </div>

                <button class="primary-btn" data-add="schedule">
                    ＋ Добавить занятие
                </button>
            </div>

            <div class="table-wrapper">
                <table id="schedule-table">
                    <thead>
                        <tr>
                            <th>День</th>
                            <th>Время</th>
                            <th>Предмет</th>
                            <th>Преподаватель</th>
                            <th>Группа</th>
                            <th>Аудитория</th>
                            <th>Тип</th>
                            <th>Действия</th>
                        </tr>
                    </thead>
                    <tbody>
        `;

        if (!scheduleCache.length) {
            html += `
                <tr>
                    <td colspan="8" class="empty-row">
                        Расписание пока не создано
                    </td>
                </tr>
            `;
        } else {
            scheduleCache.forEach(item => {
                html += `
                    <tr>
                        ${cell(dayNames[item.day_of_week] || item.day_of_week)}
                        ${cell(`${item.start_time} – ${item.end_time}`)}
                        ${cell(item.subject)}
                        ${cell(item.teacher)}
                        ${cell(item.group)}
                        ${cell(item.room)}
                        ${cell(item.lesson_type)}
                        ${actionButtons(item.id, "schedule")}
                    </tr>
                `;
            });
        }

        html += `
                    </tbody>
                </table>
            </div>
        `;

        container.innerHTML = html;
    }

    // =====================================
    // Учебные периоды и экзамены
    // =====================================

    async function loadSettings() {
        const data = await request(`${API}/settings`);
        const input = document.getElementById("maxAbsencePercent");
        if (input) input.value = data.max_absence_percent ?? "";
    }

    document.getElementById("academicSettingsForm")?.addEventListener("submit", async event => {
        event.preventDefault();
        const raw = document.getElementById("maxAbsencePercent")?.value ?? "";
        const value = raw === "" ? null : Number(raw);
        try {
            await request(`${API}/settings`, { method: "PUT", body: JSON.stringify({ max_absence_percent: value }) });
            notify("Правило посещаемости сохранено");
        } catch (error) { notify(error.message); }
    });

    async function loadPeriods() {
        periodsCache = await request(`${API}/periods`);

        const table = document.getElementById("academic-periods-table");
        if (!table) return;

        const typeNames = {
            module1: "Модуль №1",
            module2: "Модуль №2",
            exam: "Экзамен"
        };

        const rows = periodsCache.map(item => `
            <tr>
                ${cell(item.id)}
                ${cell(item.academic_year)}
                ${cell(item.semester + " семестр")}
                ${cell(typeNames[item.period_type] || item.title)}
                ${cell(item.start_date)}
                ${cell(item.end_date)}
                <td>
                    <span class="${item.is_open ? "status-active" : "status-inactive"}">
                        ${item.is_open ? "Открыт" : "Закрыт"}
                    </span>
                </td>
                <td>
                    <button
                        class="table-action edit-btn"
                        data-id="${item.id}"
                        data-type="periods">
                        Изменить
                    </button>

                    <button
                        class="table-action period-toggle-btn"
                        data-id="${item.id}"
                        data-open="${!item.is_open}">
                        ${item.is_open ? "Закрыть" : "Открыть"}
                    </button>

                    <button
                        class="table-action delete delete-btn"
                        data-id="${item.id}"
                        data-type="periods">
                        Удалить
                    </button>
                </td>
            </tr>
        `);

        fillTable(
            "academic-periods-table",
            rows,
            "Учебные периоды пока не созданы"
        );
    }

    // =====================================
    // Поля форм
    // =====================================

    function field(label, name, type = "text", required = true) {
        return `
            <div class="form-field">
                <label for="field-${name}">${label}</label>
                <input
                    id="field-${name}"
                    name="${name}"
                    type="${type}"
                    ${required ? "required" : ""}
                >
            </div>
        `;
    }

    function selectField(
        label,
        name,
        items,
        valueKey = "id",
        textKey = "name"
    ) {
        const options = items.map(item => `
            <option value="${escapeHTML(item[valueKey])}">
                ${escapeHTML(item[textKey])}
            </option>
        `).join("");

        return `
            <div class="form-field">
                <label for="field-${name}">${label}</label>
                <select id="field-${name}" name="${name}" required>
                    <option value="">Выберите...</option>
                    ${options}
                </select>
            </div>
        `;
    }

    async function loadOptions() {
        optionsCache = await request(`${API}/options`);
    }

    // =====================================
    // Открытие модального окна
    // =====================================

    async function openModal(type, id = null) {
        if (!modal || !form || !fields) {
            notify("Не найдена форма администратора");
            return;
        }

        currentType = type;
        currentId = id;

        showError("");

        let html = "";

        if (type === "faculties") {
            html += field("Название факультета", "name");
        }

        if (type === "departments") {
            if (!facultiesCache.length) {
                await loadFaculties();
            }

            html += field("Название кафедры", "name");
            html += selectField("Факультет", "faculty_id", facultiesCache);
        }

        if (type === "groups") {
            if (!departmentsCache.length) {
                await loadDepartments();
            }

            html += field("Название группы", "name");
            html += selectField("Кафедра", "department_id", departmentsCache);
            html += field("Год поступления", "admission_year", "number");
            html += field("Курс", "course_number", "number");
        }

        if (type === "subjects") {
            if (!departmentsCache.length) {
                await loadDepartments();
            }

            html += field("Название предмета", "name");
            html += field("Код предмета", "subject_code", "text", false);
            html += selectField("Кафедра", "department_id", departmentsCache);
        }

        if (type === "admins") {
            html += field("ФИО", "full_name");
            html += field("Email", "email", "email");
            if (!id) html += field("Пароль", "password", "password");
            else {
                html += field("Новый пароль (необязательно)", "password", "password", false);
                html += `<div class="form-field"><label>Статус аккаунта</label><select name="is_active" required><option value="true">Активен</option><option value="false">Заблокирован</option></select></div>`;
            }
        }

        if (type === "students") {
            if (!groupsCache.length) {
                await loadGroups();
            }

            html += field("ФИО", "full_name");
            html += field("Email", "email", "email");

            if (!id) {
                html += field("Пароль", "password", "password");
            } else {
                html += field("Новый пароль (необязательно)", "password", "password", false);
                html += `<div class="form-field"><label>Статус аккаунта</label><select name="is_active" required><option value="true">Активен</option><option value="false">Заблокирован</option></select></div>`;
            }

            html += selectField("Учебная группа", "group_id", groupsCache);
            html += field("Номер студенческого билета", "student_number");
            html += field("Год поступления", "admission_year", "number");
        }

        if (type === "teachers") {
            if (!departmentsCache.length) {
                await loadDepartments();
            }

            html += field("ФИО", "full_name");
            html += field("Email", "email", "email");

            if (!id) {
                html += field("Пароль", "password", "password");
            } else {
                html += field("Новый пароль (необязательно)", "password", "password", false);
                html += `<div class="form-field"><label>Статус аккаунта</label><select name="is_active" required><option value="true">Активен</option><option value="false">Заблокирован</option></select></div>`;
            }

            html += selectField("Кафедра", "department_id", departmentsCache);
            html += field("Номер преподавателя", "employee_number");
            html += field("Должность", "position");
        }

        if (type === "assignments") {
            await loadOptions();

            html += selectField("Преподаватель", "teacher_id", optionsCache.teachers);
            html += selectField("Предмет", "subject_id", optionsCache.subjects);
            html += selectField("Учебная группа", "group_id", optionsCache.groups);
            html += field("Учебный год", "academic_year");

            html += `
                <div class="form-field">
                    <label>Семестр</label>
                    <select name="semester" required>
                        <option value="">Выберите...</option>
                        <option value="1">1 семестр</option>
                        <option value="2">2 семестр</option>
                    </select>
                </div>
            `;
        }

        if (type === "schedule") {
            if (!assignmentsCache.length) {
                await loadAssignments();
            }

            const assignmentOptions = assignmentsCache.map(item => ({
                id: item.id,
                name: `${item.teacher || item.teacher_id} — ${item.subject || item.subject_id} — ${item.group || item.group_id}`
            }));

            html += selectField("Назначение", "assignment_id", assignmentOptions);

            html += `
                <div class="form-field">
                    <label>День недели</label>
                    <select name="day_of_week" required>
                        <option value="">Выберите...</option>
                        <option value="1">Понедельник</option>
                        <option value="2">Вторник</option>
                        <option value="3">Среда</option>
                        <option value="4">Четверг</option>
                        <option value="5">Пятница</option>
                        <option value="6">Суббота</option>
                        <option value="7">Воскресенье</option>
                    </select>
                </div>
            `;

            html += field("Время начала", "start_time", "time");
            html += field("Время окончания", "end_time", "time");
            html += field("Аудитория", "room");

            html += `
                <div class="form-field">
                    <label>Тип занятия</label>
                    <select name="lesson_type" required>
                        <option value="Лекция">Лекция</option>
                        <option value="Практика">Практика</option>
                        <option value="Лабораторная">Лабораторная</option>
                    </select>
                </div>
            `;
        }

        if (type === "periods") {
            html += field(
                "Учебный год (например, 2026-2027)",
                "academic_year"
            );

            html += `
                <div class="form-field">
                    <label>Семестр</label>
                    <select name="semester" required>
                        <option value="">Выберите семестр</option>
                        <option value="1">Первый семестр</option>
                        <option value="2">Второй семестр</option>
                    </select>
                </div>

                <div class="form-field">
                    <label>Тип периода</label>
                    <select name="period_type" required>
                        <option value="module1">Модуль №1</option>
                        <option value="module2">Модуль №2</option>
                        <option value="exam">Экзамен</option>
                    </select>
                </div>
            `;

            html += field("Название периода", "title");
            html += field("Дата начала", "start_date", "date");
            html += field("Дата окончания", "end_date", "date");

            html += `
                <div class="form-field">
                    <label>Статус периода</label>
                    <select name="is_open" required>
                        <option value="false">Закрыт</option>
                        <option value="true">Открыт</option>
                    </select>
                </div>
            `;
        }

        fields.innerHTML = html;

        const modalTitle = document.getElementById("modal-title");

        if (modalTitle) {
            modalTitle.textContent = id
                ? "Редактирование записи"
                : "Добавление записи";
        }

        modal.classList.add("open");
    }

    function closeModal() {
        if (!modal || !form) return;

        modal.classList.remove("open");
        form.reset();

        showError("");

        currentId = null;
        currentType = "";
    }

    // =====================================
    // Добавление записей
    // =====================================

    document.addEventListener("click", event => {
        const button = event.target.closest("[data-add]");

        if (!button) return;

        openModal(button.dataset.add).catch(error => {
            notify(error.message);
        });
    });

    document.getElementById("add-academic-period")
        ?.addEventListener("click", () => {
            openModal("periods").catch(error => {
                notify(error.message);
            });
        });

    // =====================================
    // Редактирование
    // =====================================

    async function editItem(type, id) {
        const maps = {
            faculties: facultiesCache,
            departments: departmentsCache,
            groups: groupsCache,
            subjects: subjectsCache,
            assignments: assignmentsCache,
            schedule: scheduleCache,
            users: usersCache,
            periods: periodsCache
        };

        const item = (maps[type] || []).find(
            element => Number(element.id) === id
        );

        if (!item) {
            notify("Данные для редактирования не найдены");
            return;
        }

        let formType = type;

        if (type === "users") {
            formType = item.role === "student"
                ? "students"
                : item.role === "teacher" ? "teachers" : "admins";
        }

        await openModal(formType, id);

        Object.entries(item).forEach(([key, value]) => {
            const input = form.elements.namedItem(key);

            if (
                input &&
                value !== null &&
                value !== undefined
            ) {
                input.value = value;
            }
        });

        if (type === "periods") {
            const status = form.elements.namedItem("is_open");

            if (status) {
                status.value = item.is_open ? "true" : "false";
            }
        }
    }

    // =====================================
    // Удаление
    // =====================================

    async function deleteItem(type, id) {
        const confirmation = type === "users"
            ? "Деактивировать аккаунт? Оценки, посещаемость и учебная история сохранятся."
            : "Вы действительно хотите удалить эту запись?";
        if (!confirm(confirmation)) {
            return;
        }

        try {
            await request(`${API}/${type}/${id}`, {
                method: "DELETE"
            });

            notify(type === "users" ? "Аккаунт деактивирован, учебная история сохранена" : "Запись успешно удалена");
            reloadCurrentSection();
        } catch (error) {
            notify(error.message);
        }
    }

    document.addEventListener("click", event => {
        const edit = event.target.closest(".edit-btn");
        const remove = event.target.closest(".delete-btn");

        if (edit) {
            editItem(
                edit.dataset.type,
                Number(edit.dataset.id)
            ).catch(error => notify(error.message));
        }

        if (remove) {
            deleteItem(
                remove.dataset.type,
                Number(remove.dataset.id)
            );
        }
    });

    // =====================================
    // Открытие и закрытие периодов
    // =====================================

    document.addEventListener("click", async event => {
        const button = event.target.closest(".period-toggle-btn");

        if (!button) return;

        const id = button.dataset.id;
        const isOpen = button.dataset.open === "true";

        try {
            await request(`${API}/periods/${id}`, {
                method: "PUT",
                body: JSON.stringify({
                    is_open: isOpen
                })
            });

            notify(isOpen ? "Период открыт" : "Период закрыт");

            await loadPeriods();
        } catch (error) {
            notify(error.message);
        }
    });

    // =====================================
    // Сохранение формы
    // =====================================

    if (form) {
        form.addEventListener("submit", async event => {
            event.preventDefault();

            showError("");

            const data = Object.fromEntries(
                new FormData(form).entries()
            );

            const numberFields = [
                "faculty_id",
                "department_id",
                "group_id",
                "admission_year",
                "course_number",
                "teacher_id",
                "subject_id",
                "semester",
                "assignment_id",
                "day_of_week"
            ];

            numberFields.forEach(key => {
                if (data[key] !== undefined && data[key] !== "") {
                    data[key] = Number(data[key]);
                }
            });

            if (data.is_open !== undefined) {
                data.is_open = data.is_open === "true";
            }
            if (data.is_active !== undefined) {
                data.is_active = data.is_active === "true";
            }

            try {
                let endpoint = currentType;

                if (
                    currentType === "students" ||
                    currentType === "teachers" ||
                    currentType === "admins"
                ) {
                    endpoint = "users";

                    data.role = currentType === "students"
                        ? "student"
                        : currentType === "teachers" ? "teacher" : "admin";
                }

                const url = currentId
                    ? `${API}/${endpoint}/${currentId}`
                    : `${API}/${endpoint}`;

                await request(url, {
                    method: currentId ? "PUT" : "POST",
                    body: JSON.stringify(data)
                });

                closeModal();

                notify("Данные успешно сохранены");

                reloadCurrentSection();
            } catch (error) {
                showError(error.message);
            }
        });
    }

    // =====================================
    // Обновление текущего раздела
    // =====================================

    function reloadCurrentSection() {
        const active = document.querySelector(".admin-section.active");

        if (!active) return;

        const name = active.id.replace("section-", "");

        loadSection(name);
    }

    // =====================================
    // Закрытие модального окна
    // =====================================

    document.getElementById("modal-close")
        ?.addEventListener("click", closeModal);

    document.getElementById("modal-cancel")
        ?.addEventListener("click", closeModal);

    modal?.addEventListener("click", event => {
        if (event.target === modal) {
            closeModal();
        }
    });

    // =====================================
    // Поиск по таблицам
    // =====================================

    document.querySelectorAll(".table-search").forEach(input => {
        input.addEventListener("input", () => {
            const table = document.getElementById(input.dataset.search);

            if (!table) return;

            const query = input.value.toLowerCase().trim();

            table.querySelectorAll("tbody tr").forEach(row => {
                row.style.display = row.textContent
                    .toLowerCase()
                    .includes(query)
                    ? ""
                    : "none";
            });
        });
    });

    // =====================================
    // Быстрые действия
    // =====================================

    document.querySelectorAll("[data-open]").forEach(button => {
        button.addEventListener("click", () => {
            showSection(button.dataset.open);
        });
    });

    // =====================================
    // Запуск панели
    // =====================================

    showSection("dashboard");
    async function loadCuratorControls(){
        try { const [groups,opts]=await Promise.all([request(`${API}/groups`),request(`${API}/options`)]);
          const gs=document.getElementById("curatorGroup"),ts=document.getElementById("curatorTeacher");
          if(gs)gs.innerHTML='<option value="">Выберите группу</option>'+groups.map(x=>`<option value="${x.id}">${escapeHTML(x.name)}</option>`).join('');
          if(ts)ts.innerHTML='<option value="">Выберите преподавателя</option>'+(opts.teachers||[]).map(x=>`<option value="${x.id}">${escapeHTML(x.name)}</option>`).join('');
        } catch(e){notify(e.message)}
    }
    document.getElementById("saveCurator")?.addEventListener("click",async()=>{try{await request(`${API}/curators`,{method:"POST",body:JSON.stringify({group_id:+document.getElementById("curatorGroup").value,teacher_id:+document.getElementById("curatorTeacher").value})});notify("Куратор назначен группе");}catch(e){notify(e.message)}});
    loadCuratorControls();
});
