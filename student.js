(() => {
  "use strict";
  const $ = id => document.getElementById(id);
  const state = { profile: null, subjects: [], schedule: [], group: [], materials: [], grades: [], attendance: [], tasks: [], portfolio: [], topics: [], curator: null, chats: [], contacts: [], activeChat: 0, aiHistory: [], notifications: [], selectedPeriod: "all" };
  const api = async (path, options = {}) => {
    const res = await fetch(`/api/portal/student/${path}`, { credentials: "same-origin", ...options });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || "Не удалось загрузить данные");
    return data;
  };
  const esc = value => String(value ?? "").replace(/[&<>"']/g, c => ({
    "&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"
  }[c]));
  const initials = name => String(name || "Студент").trim().split(/\s+/).slice(0,2).map(x=>x[0]||"").join("").toUpperCase();
  const empty = message => `<div class="panel empty-state"><p>${esc(message)}</p></div>`;
  const notify = msg => {
    const node = $("toast");
    if (!node) return;
    node.textContent = msg; node.classList.add("show");
    setTimeout(() => node.classList.remove("show"), 2600);
  };
  function navigate(page) {
    document.querySelectorAll(".page").forEach(x => x.classList.toggle("active", x.id === `page-${page}`));
    document.querySelectorAll(".nav-item").forEach(x => x.classList.toggle("active", x.dataset.page === page));
    const title = document.querySelector(`[data-page="${page}"]`);
    if ($("crumbName") && title) $("crumbName").textContent = title.textContent.trim();
    $("sidebar")?.classList.remove("open");
    window.scrollTo({top:0, behavior:"smooth"});
  }
  document.querySelectorAll(".nav-item").forEach(x => x.addEventListener("click", () => navigate(x.dataset.page)));
  document.querySelectorAll("[data-goto]").forEach(x => x.addEventListener("click", () => navigate(x.dataset.goto)));
  $("menuBtn")?.addEventListener("click", () => $("sidebar")?.classList.toggle("open"));
  if ($("todayDate")) $("todayDate").textContent = new Date().toLocaleDateString("ru-RU", {day:"numeric",month:"long",year:"numeric"});

  function renderProfile() {
    const p = state.profile || {};
    const name = p.full_name || "Студент";
    const parts = name.split(/\s+/);
    const group = p.group_name || "Группа не назначена";
    ["miniName","dashboardName","profileDisplayName"].forEach(id => { if ($(id)) $(id).textContent = name; });
    if ($("welcomeName")) $("welcomeName").textContent = parts[1] || parts[0] || "студент";
    ["miniAvatar","topAvatar","dashboardAvatar","profileAvatar"].forEach(id => { if ($(id)) $(id).textContent = initials(name); });
    if ($("profileDisplayName")) $("profileDisplayName").textContent = name;
    if ($("lastName")) $("lastName").value = parts[0] || "";
    if ($("firstName")) $("firstName").value = parts[1] || "";
    if ($("middleName")) $("middleName").value = parts.slice(2).join(" ");
    if ($("email")) $("email").value = p.email || "";
    const dashboardRows = document.querySelectorAll("#page-dashboard .info-row b");
    if (dashboardRows.length >= 3) {
      dashboardRows[0].textContent = p.faculty || "—";
      dashboardRows[1].textContent = group;
      dashboardRows[2].textContent = p.admission_year || "—";
    }
    ["birthDate","phone","address"].forEach(id => { if ($(id)) { $(id).value = ""; $(id).readOnly = true; } });
    ["lastName","firstName","middleName","email"].forEach(id => { if ($(id)) $(id).readOnly = true; });
    const mini = $("miniName")?.parentElement?.querySelector("small");
    if (mini) mini.textContent = `Студент · ${group}`;
    const curator = $("curatorInfo");
    if (curator) curator.textContent = state.curator?.full_name ? `${state.curator.full_name} · ${state.curator.position || "Куратор"} · ${state.curator.email}` : "Куратор пока не назначен администратором.";
    const groupBanner = document.querySelector(".group-banner");
    if (groupBanner) {
      const h = groupBanner.querySelector("h2"); if (h) h.textContent = group;
      const desc = groupBanner.querySelector("p"); if (desc) desc.textContent = p.department || "Кафедра пока не назначена";
      const count = groupBanner.querySelector(".group-count b"); if (count) count.textContent = String(state.group.length);
    }
    document.querySelectorAll(".profile-detail").forEach((node, i) => {
      const b = node.querySelector("b");
      if (b) b.textContent = [p.faculty || "—", p.admission_year ? `Поступление: ${p.admission_year}` : "—", group][i] || "—";
    });
  }
  function getVisibleSubjects() {
    if (state.selectedPeriod === "all") return state.subjects;
    const [year, semester] = state.selectedPeriod.split("|");
    return state.subjects.filter(s => s.academic_year === year && String(s.semester) === semester);
  }
  function renderSubjects() {
    const subjects = getVisibleSubjects();
    const quick = document.querySelector(".quick-grid");
    if (quick) quick.innerHTML = `
      <button class="quick-card blue-card" data-goto="session"><span class="quick-icon">▤</span><small>ПРЕДМЕТЫ</small><strong>${subjects.length}</strong><span>Перейти к предметам →</span></button>
      <button class="quick-card green-card" data-goto="attendance"><span class="quick-icon">◷</span><small>ПОСЕЩАЕМОСТЬ</small><strong>—</strong><span>Данные пока не внесены →</span></button>
      <button class="quick-card violet-card" data-goto="materials"><span class="quick-icon">▧</span><small>УЧЕБНЫЕ МАТЕРИАЛЫ</small><strong>—</strong><span>Открыть материалы →</span></button>
      <button class="quick-card orange-card" data-goto="group"><span class="quick-icon">♧</span><small>МОЯ ГРУППА</small><strong>${esc(state.profile?.group_name || "—")}</strong><span>${state.group.length} студентов →</span></button>`;
    quick?.querySelectorAll("[data-goto]").forEach(x=>x.addEventListener("click",()=>navigate(x.dataset.goto)));
    const calcSelect = $("gradeCalcSubject");
    if (calcSelect) calcSelect.innerHTML = subjects.map(s => `<option value="${s.assignment_id}">${esc(s.subject)}</option>`).join("");
    renderGradeCalculator();
    const target = $("subjectScoreList");
    if (target) target.innerHTML = subjects.length ? subjects.map((s,i) => {
      const g = state.grades.find(x => x.assignment_id === s.assignment_id);
      const m1 = g?.has_module1 ? `${Number(g.module1).toFixed(1)} / 25` : "Не выставлен";
      const m2 = g?.has_module2 ? `${Number(g.module2).toFixed(1)} / 25` : "Не выставлен";
      return `<article class="subject-score-card"><span class="subject-index">${i+1}.</span><span class="subject-name">${esc(s.subject)}</span><span class="exam-caption">${esc(s.academic_year)} · ${s.semester} семестр</span><span class="score-grid"><span>Модуль 1<strong>${m1}</strong></span><span>Модуль 2<strong>${m2}</strong></span><span>Преподаватель<strong>${esc(s.teacher)}</strong></span></span><span class="subject-open">Сумма модулей: ${g ? Number(g.total).toFixed(1) + " / 50" : "нет данных"}</span></article>`;
    }).join("") : empty("Предметы пока не назначены администратором.");
    if ($("sessionSubjectCount")) $("sessionSubjectCount").textContent = String(subjects.length);
    const overview = document.querySelector(".session-overview");
    if (overview) {
      const currentGrades = state.grades.filter(g => subjects.some(s => s.assignment_id === g.assignment_id) && (g.has_module1 || g.has_module2));
      const average = currentGrades.length ? currentGrades.reduce((sum,g) => sum + Number(g.total),0) / currentGrades.length : null;
      overview.innerHTML = `<div><small>ПРЕДМЕТОВ В СЕМЕСТРЕ</small><b>${subjects.length}</b></div><div><small>СРЕДНЯЯ СУММА МОДУЛЕЙ</small><b>${average === null ? "—" : average.toFixed(1) + " / 50"}</b></div><div><small>ПРЕДМЕТОВ С ОЦЕНКАМИ</small><b>${currentGrades.length}</b></div>`;
    }
    const note = document.querySelector("#page-session > .muted-note");
    if (note) note.textContent = "Оценки появятся после того, как преподаватели внесут их в электронный журнал.";
  }
  function renderGradeCalculator() {
    const result=$("gradeCalcResult"), select=$("gradeCalcSubject"), input=$("gradeCalcTarget"); if(!result||!select||!input)return;
    const g=state.grades.find(x=>String(x.assignment_id)===select.value); if(!g){result.textContent="Для выбранного предмета пока нет данных журнала.";return;}
    const target=Math.max(0,Math.min(50,Number(input.value)||0));
    if(g.has_module2){const diff=target-Number(g.total);result.textContent=`Уже набрано ${Number(g.total).toFixed(1)} из 50. ${diff<=0?"Цель достигнута.":`До желаемого результата не хватает ${diff.toFixed(1)} балла.`}`;return;}
    const required=Math.max(0,target-(g.has_module1?Number(g.module1):0));
    result.textContent=required>25?`Для цели ${target} баллов потребуется ${required.toFixed(1)} балла за второй модуль — это выше максимума 25. Пересмотрите цель.`:`Для суммы ${target} баллов нужно набрать ${required.toFixed(1)} из 25 за второй модуль.`;
  }
  $("gradeCalcSubject")?.addEventListener("change",renderGradeCalculator);
  $("gradeCalcTarget")?.addEventListener("input",renderGradeCalculator);

  function renderMaterials() {
    const select = $("materialSubject");
    if (select) select.innerHTML = state.subjects.map(s => `<option value="${s.assignment_id}">${esc(s.subject)}</option>`).join("");
    const subject = state.subjects.find(x => String(x.assignment_id) === select?.value);
    if ($("materialSubjectName")) $("materialSubjectName").textContent = subject?.subject || "Предметы не назначены";
    const banner = document.querySelector(".subject-banner small");
    if (banner) banner.textContent = subject ? `Преподаватель: ${subject.teacher} · № ${subject.teacher_employee_number}` : "Материалы появятся после назначения предметов.";
    const selectedType = $("materialType")?.value || "all";
    const search = ($("materialSearch")?.value || "").trim().toLowerCase();
    const list = state.materials.filter(x => (!subject || x.subject === subject.subject) && (selectedType === "all" || x.type === selectedType) && (!search || `${x.title} ${x.file_name} ${x.subject}`.toLowerCase().includes(search)));
    const syllabus = state.materials.find(x => (!subject || x.subject === subject.subject) && x.type === "syllabus");
    const syllabusButton = $("syllabusOpen");
    if (syllabusButton) { syllabusButton.disabled = !syllabus; syllabusButton.dataset.url = syllabus?.url || ""; }
    const topics = state.topics.filter(x => !subject || x.assignment_id === subject.assignment_id);
    const topicsHTML = topics.length ? `<h3 class="materials-subheading">Темы курса</h3>` + topics.map(x => `<article class="material-card"><b>${esc(x.title)}</b><small>${esc(x.subject)}</small><p>${esc(x.description)}</p></article>`).join("") : "";
    const materialsHTML = list.length ? `<h3 class="materials-subheading">Файлы и материалы</h3>` + list.map(x => `<article class="material-card"><b>${esc(x.title)}</b><small>${esc(x.subject)} · ${esc(x.teacher)} · № ${esc(x.teacher_number)}</small><a href="${esc(x.url)}" target="_blank" rel="noopener">Открыть / скачать: ${esc(x.file_name)}</a></article>`).join("") : "";
    if ($("topicsContainer")) $("topicsContainer").innerHTML = topicsHTML + materialsHTML || empty(subject ? "Преподаватель пока не добавил темы и материалы по этому предмету." : "Материалы пока недоступны: предметы не назначены.");
    if ($("syllabusOpen")) $("syllabusOpen").disabled = true;
    updateDashboardQuickCards();
  }
  function renderSchedule() {
    const names = ["","Понедельник","Вторник","Среда","Четверг","Пятница","Суббота","Воскресенье"];
    const colors = ["#e94b35","#16a765","#3984ee","#a34bea","#e43d8f","#12a99a"];
    const target = $("scheduleGrid");
    if (target) {
      if (!state.schedule.length) target.innerHTML = empty("Расписание пока не составлено администратором.");
      else {
        const grouped = {}; state.schedule.forEach(x => (grouped[x.day_of_week] ||= []).push(x));
        target.innerHTML = `<div class="week-cards">${Object.keys(grouped).sort((a,b)=>a-b).map(day => `<section class="day-column"><h3>${names[day] || ""}</h3>${grouped[day].sort((a,b)=>String(a.start_time).localeCompare(String(b.start_time))).map((x,i)=>`<article class="color-lesson" style="--lesson-color:${colors[(x.id+i)%colors.length]}"><b>${esc(x.start_time)} – ${esc(x.end_time)}</b><strong>${esc(x.subject)}</strong><span>${esc(x.lesson_type)} · ${esc(x.room)}</span><small>Преподаватель: ${esc(x.teacher)} · № ${esc(x.teacher_employee_number)}</small></article>`).join("")}</section>`).join("")}</div>`;
      }
    }

    // PostgreSQL stores Monday as 1 ... Sunday as 7. JS uses Sunday=0 ... Saturday=6.
    const jsDay = new Date().getDay();
    const today = jsDay === 0 ? 7 : jsDay;
    const todayRows = state.schedule
      .filter(x => Number(x.day_of_week) === today)
      .sort((a,b) => String(a.start_time).localeCompare(String(b.start_time)));
    const firstPanel = document.querySelector("#page-dashboard .two-columns .panel");
    if (firstPanel) {
      firstPanel.querySelectorAll(".lesson, .today-empty").forEach(x=>x.remove());
      const h = firstPanel.querySelector(".panel-title");
      const subtitle = h?.querySelector("p");
      if (subtitle) subtitle.textContent = `Расписание на сегодня · ${names[today]}`;
      const html = todayRows.length
        ? todayRows.map(x=>`<div class="lesson"><div class="lesson-time">${esc(x.start_time)}<small>${esc(x.end_time)}</small></div><div class="lesson-line blue-line"></div><div class="lesson-info"><b>${esc(x.subject)}</b><small>${esc(x.lesson_type)} · Аудитория ${esc(x.room)}</small></div></div>`).join("")
        : `<p class="muted-note today-empty">${today === 7 ? "Сегодня воскресенье — занятий по расписанию нет." : "На сегодня занятия не назначены."}</p>`;
      if (h) h.insertAdjacentHTML("afterend", html);
    }
  }

  function updateDashboardQuickCards() {
    const cards = document.querySelectorAll("#page-dashboard .quick-card");
    if (cards.length < 4) return;
    const totalLessons = state.attendance.reduce((sum, x) => sum + Number(x.lessons || 0), 0);
    const presentLessons = state.attendance.reduce((sum, x) => sum + Number(x.present || 0), 0);
    const attendancePercent = totalLessons ? Math.round(presentLessons / totalLessons * 100) : null;
    const attendanceStrong = cards[1].querySelector("strong");
    const attendanceText = cards[1].querySelector("span:last-child");
    if (attendanceStrong) attendanceStrong.textContent = attendancePercent === null ? "—" : `${attendancePercent}%`;
    if (attendanceText) attendanceText.textContent = totalLessons ? `${presentLessons} из ${totalLessons} занятий посещено →` : "Отметки пока не внесены →";

    const materialCount = state.materials.length + state.topics.length;
    const materialsStrong = cards[2].querySelector("strong");
    const materialsText = cards[2].querySelector("span:last-child");
    if (materialsStrong) materialsStrong.textContent = String(materialCount);
    if (materialsText) materialsText.textContent = materialCount ? "Материалов и тем курса →" : "Материалы пока не добавлены →";
  }
  function renderGroup() {
    if ($("studentList")) $("studentList").innerHTML = state.group.length ? state.group.map(x =>
      `<div class="student-row"><span class="avatar">${esc(initials(x.full_name))}</span><div><b>${esc(x.full_name)}</b><small>№ ${esc(x.student_number)}</small></div></div>`
    ).join("") : empty("Группа не назначена или в ней пока нет студентов.");
    $("groupSearch")?.addEventListener("input", e => {
      const q = e.target.value.toLowerCase();
      document.querySelectorAll("#studentList .student-row").forEach(row => row.hidden = !row.textContent.toLowerCase().includes(q));
    });
  }
  function renderEmptyAttendance() {
    const totals = state.attendance.reduce((a,x) => ({lessons:a.lessons+x.lessons,present:a.present+x.present,absent:a.absent+x.absent}), {lessons:0,present:0,absent:0});
    const percent = totals.lessons ? Math.round(totals.present / totals.lessons * 100) : 0;
    const summary = document.querySelector("#page-attendance .grade-summary");
    if (summary) summary.innerHTML = `<article class="summary-card"><small>ПОСЕЩАЕМОСТЬ</small><strong>${totals.lessons ? percent + "%" : "—"}</strong><p>По всем предметам</p></article><article class="summary-card"><small>ПОСЕЩЕНО</small><strong>${totals.present}</strong><p>Занятий</p></article><article class="summary-card"><small>ПРОПУЩЕНО</small><strong>${totals.absent}</strong><p>НБ · без уважительной причины</p></article>`;
    const list = document.querySelector("#page-attendance .attendance-list");
    if (list) list.innerHTML = state.attendance.length ? state.attendance.map(x => `<article class="attendance-row"><b>${esc(x.subject)}</b><span>Посещаемость: ${Math.round(x.percent)}%</span><span>Был: ${x.present}</span><span>НБ: ${x.absent}</span><span>Уважительно: ${x.excused + x.sick}</span><span>${x.max_absence_percent == null ? "Лимит пропусков не установлен" : x.risk ? "Внимание: превышен лимит НБ" : `Лимит НБ: ${Number(x.max_absence_percent).toFixed(1)}%`}</span></article>`).join("") : `<p class="muted-note">Данные о посещаемости появятся после заполнения преподавателем электронного журнала.</p>`;
    updateDashboardQuickCards();
  }
  function renderStudentNotifications() {
    const target = $("studentNotifications"); if (!target) return;
    target.innerHTML = state.notifications.length ? state.notifications.map(n => {
      const action = n.read_at ? "" : `<button type="button" class="outline-button" data-notification-read="${n.id}">Прочитано</button>`;
      return `<article class="notification-item ${n.read_at ? "read" : "unread"}"><div><b>${esc(n.title)}</b><p>${esc(n.body)}</p><small>${new Date(n.created_at).toLocaleString("ru-RU")}</small></div>${action}</article>`;
    }).join("") : empty("Новых уведомлений пока нет.");
  }
  $("studentNotifications")?.addEventListener("click",async e=>{const id=Number(e.target.dataset.notificationRead);if(!id)return;try{await api("notifications?id="+id,{method:"POST"});state.notifications=await api("notifications");renderStudentNotifications();}catch(err){notify(err.message);}});

  function renderAI() {
    const select=$("aiMaterial"); if(select) { const supported=state.materials.filter(m=>/\.(pdf|docx|txt)$/i.test(m.file_name||"")); select.innerHTML=`<option value="0">Без прикреплённого материала</option>`+supported.map(m=>`<option value="${m.id}">${esc(m.subject)} · ${esc(m.title)} (${esc(m.file_name)})</option>`).join(""); }
    const target=$("aiConversation"); if(target) target.innerHTML=state.aiHistory.length?state.aiHistory.map(m=>`<article class="ai-message ${m.role}"><b>${m.role==="user"?"Вы":"AI-помощник"}</b><p>${esc(m.content)}</p><small>${new Date(m.created_at).toLocaleString("ru-RU")}</small></article>`).join(""):empty("Задайте первый вопрос помощнику.");
  }
  $("aiForm")?.addEventListener("submit",async e=>{e.preventDefault();const button=e.target.querySelector('button[type="submit"]');try{const message=$("aiQuestion").value.trim();if(!message)return;button.disabled=true;button.textContent="AI анализирует…";const result=await api("ai",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({message,material_id:Number($("aiMaterial").value)})});$("aiQuestion").value="";state.aiHistory=await api("ai");renderAI();notify("Ответ получен");}catch(err){notify(err.message);}finally{button.disabled=false;button.textContent="Отправить вопрос";}});

  function renderStudentChats() {
    const list=$("studentChatList"); if(list) list.innerHTML=state.chats.length?state.chats.map(c=>`<button type="button" class="chat-list-item ${state.activeChat===c.id?"active":""}" data-chat-id="${c.id}"><b>${esc(c.name)}</b><small>${esc(c.last_message||"Нет сообщений")}</small>${c.unread?`<span class="chat-unread">${c.unread}</span>`:""}</button>`).join(""):empty("Чатов пока нет.");
    const select=$("messageContact"); if(select) select.innerHTML=`<option value="">Выберите собеседника</option>`+state.contacts.map(c=>`<option value="${c.id}">${esc(c.name)} · ${c.role==="teacher"?"преподаватель":"студент"}</option>`).join("");
  }
  async function loadStudentChats(){[state.chats,state.contacts]=await Promise.all([api("messages"),api("messages?contacts=1")]);renderStudentChats();}
  async function openStudentChat(id,name){state.activeChat=id;$("studentChatTitle").textContent=name||"Чат";const messages=await api("messages?conversation_id="+id);$("studentChatMessages").innerHTML=messages.map(m=>`<article class="chat-message ${m.sender_id===state.profile?.id?"mine":""}"><b>${esc(m.sender_name)}</b><p>${esc(m.body)}</p><small>${new Date(m.created_at).toLocaleString("ru-RU")}</small></article>`).join("")||empty("Сообщений пока нет.");renderStudentChats();}
  $("studentChatList")?.addEventListener("click",e=>{const id=Number(e.target.closest("[data-chat-id]")?.dataset.chatId);if(!id)return;const chat=state.chats.find(x=>x.id===id);openStudentChat(id,chat?.name).catch(err=>notify(err.message));});
  $("startChat")?.addEventListener("click",async()=>{try{const recipient=Number($("messageContact").value),body=$("firstMessage").value.trim();if(!recipient||!body)throw Error("Выберите собеседника и напишите сообщение");const result=await api("messages",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({recipient_id:recipient,body})});$("firstMessage").value="";await loadStudentChats();const chat=state.chats.find(x=>x.id===result.conversation_id);await openStudentChat(result.conversation_id,chat?.name||"Новый чат");}catch(err){notify(err.message);}});
  $("studentMessageForm")?.addEventListener("submit",async e=>{e.preventDefault();try{if(!state.activeChat)throw Error("Сначала выберите чат");const body=$("studentMessageText").value.trim();if(!body)return;await api("messages",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({conversation_id:state.activeChat,body})});$("studentMessageText").value="";const chat=state.chats.find(x=>x.id===state.activeChat);await openStudentChat(state.activeChat,chat?.name);await loadStudentChats();}catch(err){notify(err.message);}});

  function renderTasks() {
    const target = $("personalTasksList"); if (!target) return;
    const planSelect=$("studyPlanSubject"); if(planSelect) planSelect.innerHTML=state.subjects.map(s=>`<option value="${esc(s.subject)}">${esc(s.subject)}</option>`).join("");
    target.innerHTML = state.tasks.length ? state.tasks.map(t => `<article class="personal-task ${t.is_completed ? "is-completed" : ""}"><label><input type="checkbox" data-task-toggle="${t.id}" ${t.is_completed ? "checked" : ""}> <b>${esc(t.title)}</b></label><p>${esc(t.description)}</p><small>${t.due_at ? "Срок: " + new Date(t.due_at).toLocaleString("ru-RU") : "Без срока"} · Приоритет: ${["","Высокий","Обычный","Низкий"][t.priority] || "Обычный"}</small><button type="button" class="outline-button" data-task-delete="${t.id}">Удалить</button></article>`).join("") : empty("Пока нет задач. Добавьте первую задачу выше.");
  }
  function renderPortfolio() {
    const target = $("portfolioItems"); if (!target) return;
    const publicLink = $("publicPortfolioLink");
    if (publicLink && state.profile?.id) publicLink.href = `/portfolio/${state.profile.id}`;
    const names = {project:"Проект",certificate:"Сертификат",lab:"Лабораторная",achievement:"Достижение",skill:"Навык",other:"Другое"};
    target.innerHTML = state.portfolio.length ? state.portfolio.map(x => `<article class="portfolio-item"><small>${names[x.category] || "Работа"} · ${x.is_public ? "Публичная запись" : "Только мне"}</small><h3>${esc(x.title)}</h3><p>${esc(x.description)}</p>${x.skills ? `<div class="portfolio-skills">${esc(x.skills)}</div>` : ""}${x.resource_url ? `<a href="${esc(x.resource_url)}" target="_blank" rel="noopener">Открыть работу / сертификат ↗</a>` : ""}<button type="button" class="outline-button" data-portfolio-delete="${x.id}">Удалить</button></article>`).join("") : empty("Портфолио пока пустое. Добавьте проект, лабораторную или сертификат.");
  }
  async function loadPersonalFeatures() {
    [state.tasks, state.portfolio] = await Promise.all([api("tasks"), api("portfolio"), api("topics"), api("curator"), api("messages"), api("messages?contacts=1"), api("ai"), api("notifications")]);
    renderTasks(); renderPortfolio(); renderStudentChats(); renderAI(); renderStudentNotifications();
  }
  $("studyPlanForm")?.addEventListener("submit",async e=>{e.preventDefault();try{const topics=$("studyPlanTopics").value.split(/\r?\n/).map(x=>x.trim()).filter(Boolean);const result=await api("study-plan",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({subject:$("studyPlanSubject").value,exam_date:$("studyPlanExamDate").value,topics})});await loadPersonalFeatures();notify(`План создан: добавлено задач — ${result.created_tasks}`);$("studyPlanTopics").value="";}catch(err){notify(err.message);}});
  $("personalTaskForm")?.addEventListener("submit", async e => {
    e.preventDefault();
    try {
      const due = $("taskDueAt").value;
      await api("tasks", {method:"POST", headers:{"Content-Type":"application/json"}, body:JSON.stringify({title:$("taskTitle").value,description:$("taskDescription").value,due_at:due ? new Date(due).toISOString() : "",priority:Number($("taskPriority").value)})});
      e.target.reset(); $("taskPriority").value="2"; await loadPersonalFeatures(); notify("Задача добавлена");
    } catch(err) { notify(err.message); }
  });
  $("personalTasksList")?.addEventListener("change", async e => {
    const id = Number(e.target.dataset.taskToggle); if (!id) return;
    const task = state.tasks.find(x => x.id === id); if (!task) return;
    try { await api("tasks?id="+id,{method:"PUT",headers:{"Content-Type":"application/json"},body:JSON.stringify({title:task.title,description:task.description,due_at:task.due_at || "",priority:task.priority,is_completed:e.target.checked})}); await loadPersonalFeatures(); }
    catch(err) { notify(err.message); }
  });
  $("personalTasksList")?.addEventListener("click", async e => {
    const id=Number(e.target.dataset.taskDelete); if(!id)return;
    try { await api("tasks?id="+id,{method:"DELETE"}); await loadPersonalFeatures(); notify("Задача удалена"); } catch(err) { notify(err.message); }
  });
  $("portfolioForm")?.addEventListener("submit", async e => {
    e.preventDefault();
    try {
      await api("portfolio",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({title:$("portfolioTitle").value,description:$("portfolioDescription").value,category:$("portfolioCategory").value,skills:$("portfolioSkills").value,resource_url:$("portfolioURL").value,is_public:$("portfolioPublic").checked})});
      e.target.reset(); await loadPersonalFeatures(); notify("Работа добавлена в портфолио");
    } catch(err) { notify(err.message); }
  });
  $("portfolioItems")?.addEventListener("click", async e => {
    const id=Number(e.target.dataset.portfolioDelete); if(!id)return;
    try { await api("portfolio?id="+id,{method:"DELETE"}); await loadPersonalFeatures(); notify("Запись удалена"); } catch(err) { notify(err.message); }
  });

  function renderSemesterOptions() {
    const select=$("gradeSemester"); if(!select)return;
    const periods=[...new Map(state.subjects.map(s=>[`${s.academic_year}|${s.semester}`,`${s.academic_year} · ${s.semester} семестр`])).entries()];
    select.innerHTML=`<option value="all">Все учебные периоды</option>`+periods.map(([value,label])=>`<option value="${esc(value)}">${esc(label)}</option>`).join("");
    select.value=state.selectedPeriod;
  }
  $("gradeSemester")?.addEventListener("change",e=>{state.selectedPeriod=e.target.value;renderSubjects();});

  async function init() {
    try {
      const results = await Promise.all([
        api("profile"), api("subjects"), api("schedule"), api("group"), api("materials"), api("grades"), api("attendance"), api("tasks"), api("portfolio"), api("topics"), api("curator"), api("messages"), api("messages?contacts=1"), api("ai"), api("notifications")
      ]);
      [state.profile,state.subjects,state.schedule,state.group,state.materials,state.grades,state.attendance,state.tasks,state.portfolio,state.topics,state.curator,state.chats,state.contacts,state.aiHistory,state.notifications] = results;
      renderProfile(); renderSemesterOptions(); renderSubjects(); renderMaterials(); renderSchedule(); renderGroup(); renderEmptyAttendance(); renderTasks(); renderPortfolio(); renderStudentChats(); renderAI(); renderStudentNotifications();
    } catch (e) { notify(e.message); }
  }
  $("materialSubject")?.addEventListener("change", renderMaterials);
  $("materialType")?.addEventListener("change", renderMaterials);
  $("materialSearch")?.addEventListener("input", renderMaterials);
  $("syllabusOpen")?.addEventListener("click", e => { const url=e.currentTarget.dataset.url; if(url) window.open(url,"_blank","noopener"); });
  $("profileForm")?.addEventListener("submit", e => { e.preventDefault(); notify("Личные данные изменяются через администратора. Обратитесь в деканат."); });
  $("modalClose")?.addEventListener("click", () => $("modal")?.classList.remove("open"));
  $("modal")?.addEventListener("click", e => { if (e.target === $("modal")) $("modal").classList.remove("open"); });
  init();
})();
