import { pb, isAuthenticated, getCurrentUser, login, register, logout, fetchTaskDetails } from './api.js';

// === ГЛОБАЛЬНЫЙ СТЕЙТ ПРИЛОЖЕНИЯ ===
const state = {
    tasks: new Map(),       // Все задачи в виде Map(id -> task)
    activeTaskId: null,     // ID выбранной в данный момент задачи
};

// === ИНИЦИАЛИЗАЦИЯ ПРИ СТАРТЕ ===
document.addEventListener('DOMContentLoaded', async () => {
    initDOMListeners();
    checkSession();
});

// Проверка сессии и переключение экранов
function checkSession() {
    const overlay = document.getElementById('auth-overlay');
    const authStatus = document.getElementById('auth-status');

    if (isAuthenticated()) {
        overlay.classList.add('hidden');
        const user = getCurrentUser();
        authStatus.textContent = `👤 ${user.email}`;
        loadInitialData(); // Загружаем данные только после успешного входа
    } else {
        overlay.classList.remove('hidden');
        authStatus.textContent = '';
        document.getElementById('tasks-tree-container').innerHTML = '<p style="color: #666;">Требуется авторизация...</p>';
    }
}

// === РАБОТА С ДАННЫМИ (ЗАГРУЗКА И REALTIME) ===
async function loadInitialData() {
    try {
        // Очищаем стейт
        state.tasks.clear();

        // 1. Скачиваем плоский список задач из PocketBase
        const rawTasks = await pb.collection('tasks').getFullList({
            sort: 'created'
        });

        // 2. Наполняем наш Map
        rawTasks.forEach(task => {
            state.tasks.set(task.id, {
                ...task,
                children: [] // Слот под ID подзадач
            });
        });

        // 3. Строим связи (вычисляем детей для каждого родителя)
        state.tasks.forEach(task => {
            if (task.parent_id && state.tasks.has(task.parent_id)) {
                state.tasks.get(task.parent_id).children.push(task.id);
            }
        });

        // Отрисовываем интерфейс
        renderTree();

        // 4. Включаем Realtime-подписку на изменения в БД
        pb.collection('tasks').subscribe('*', handleRealtimeEvent);

    } catch (error) {
        console.error("Ошибка инициализации данных:", error);
    }
}

// Обработчик вебсокетов от PocketBase
function handleRealtimeEvent(e) {
    console.log('Событие БД в реальном времени:', e.action, e.record);

    if (e.action === 'create' || e.action === 'update') {
        // Обновляем или добавляем задачу в плоскую карту
        const existingTask = state.tasks.get(e.record.id) || { children: [] };
        state.tasks.set(e.record.id, {
            ...existingTask,
            ...e.record
        });

        // Если это новая задача с родителем, перестраиваем связи
        if (e.action === 'create' && e.record.parent_id && state.tasks.has(e.record.parent_id)) {
            const parent = state.tasks.get(e.record.parent_id);
            if (!parent.children.includes(e.record.id)) {
                parent.children.push(e.record.id);
            }
        }
    } else if (e.action === 'delete') {
        state.tasks.delete(e.record.id);
        // Удаляем упоминание из родительских списков
        state.tasks.forEach(t => {
            t.children = t.children.filter(id => id !== e.record.id);
        });
        if (state.activeTaskId === e.record.id) state.activeTaskId = null;
    }

    // Перерисовываем дерево и детали при любом изменении на бэкенде
    renderTree();
    if (state.activeTaskId) showDetails(state.activeTaskId);
}

// === ДЕКЛАРАТИВНЫЙ РЕНДЕРИНГ ДЕРЕВА ===
function renderTree() {
    const container = document.getElementById('tasks-tree-container');
    container.innerHTML = '';

    if (state.tasks.size === 0) {
        container.innerHTML = '<p style="color: #666;">Задач пока нет. Создайте их в админке.</p>';
        return;
    }

    // Находим корневые задачи (у которых нет родителя или родитель не найден в текущем списке)
    const rootTasks = Array.from(state.tasks.values()).filter(t => !t.parent_id || !state.tasks.has(t.parent_id));

    // Запускаем рекурсию
    rootTasks.forEach(task => {
        container.appendChild(createTaskDOMNode(task, 0));
    });
}

// Рекурсивное создание DOM-элементов
function createTaskDOMNode(task, depth) {
    const wrapper = document.createElement('div');
    wrapper.className = `task-item ${task.status === 'done' ? 'completed' : ''} ${task.status === 'failed' ? 'blocked' : ''}`;

    // Подсвечиваем активную задачу
    const isActive = task.id === state.activeTaskId ? 'active' : '';

    const row = document.createElement('div');
    row.className = `task-row ${isActive}`;
    row.style.paddingLeft = `${depth * 20 + 8}px`; // Сдвиг для имитации дерева
    row.dataset.id = task.id;

    // Бейдж исполнителя
    const badgeClass = task.assignee_type === 'ai' ? 'badge-ai' : 'badge-human';
    const badgeText = task.assignee_type === 'ai' ? '🤖 AI' : '👤 Human';

    row.innerHTML = `
        <input type="checkbox" ${task.status === 'done' ? 'checked' : ''} data-id="${task.id}" class="task-checkbox">
        <span style="margin-left: 8px;">${task.title}</span>
        <span class="badge ${badgeClass}">${badgeText}</span>
    `;

    wrapper.appendChild(row);

    // Рекурсивно добавляем детей
    task.children.forEach(childId => {
        const childTask = state.tasks.get(childId);
        if (childTask) {
            wrapper.appendChild(createTaskDOMNode(childTask, depth + 1));
        }
    });

    return wrapper;
}

// === ПОДГРУЗКА И ОТОБРАЖЕНИЕ ДЕТАЛЕЙ ЗАДАЧИ ===
async function showDetails(taskId) {
    state.activeTaskId = taskId;

    // Перерисовываем дерево, чтобы обновить класс .active у строк
    renderTree();

    const titleEl = document.getElementById('details-title');
    const contentEl = document.getElementById('details-content');

    const task = state.tasks.get(taskId);
    titleEl.textContent = task ? task.title : 'Детали задачи';
    contentEl.innerHTML = '<p style="color: #666;">Загрузка подробностей...</p>';

    try {
        // Ленивая (Lazy) загрузка тяжелых данных с сервера по требованию
        const details = await fetchTaskDetails(taskId);

        let filesHtml = '';
        if (details.files.length > 0) {
            filesHtml = `
                <h4>📎 Прикрепленные архивы / файлы:</h4>
                <ul>
                    ${details.files.map(f => `<li><a href="${f.url}" target="_blank" download>${f.name}</a></li>`).join('')}
                </ul>
            `;
        }

        contentEl.innerHTML = `
            <div style="line-height: 1.6;">
                <strong>Статус:</strong> ${task.status.toUpperCase()}<br>
                <strong>Исполнитель:</strong> ${task.assignee_type === 'ai' ? 'ИИ Агент' : 'Человек'}<br>
                <hr style="border: 0; border-top: 1px solid var(--border-color); margin: 15px 0;">
                
                <h4>📝 Подробное описание (Markdown):</h4>
                <div style="background: #fafbfc; padding: 10px; border: 1px solid var(--border-color); border-radius: 4px;">
                    ${details.description.replace(/\n/g, '<br>')}
                </div>

                <h4>📊 Структурированные метаданные (JSON):</h4>
                <pre>${JSON.stringify(details.metaJson, null, 2)}</pre>

                ${filesHtml}
            </div>
        `;
    } catch (error) {
        contentEl.innerHTML = `<p style="color: var(--blocked-color);">Не удалось загрузить детали задачи.</p>`;
    }
}

// === НАСТРОЙКА СЛУШАТЕЛЕЙ DOM (ИВЕНТЫ) ===
function initDOMListeners() {
    // 1. Форма входа / Регистрации
    const authForm = document.getElementById('auth-html-form');
    const toggleBtn = document.getElementById('toggle-auth-mode');
    let isLoginMode = true;

    toggleBtn.addEventListener('click', () => {
        isLoginMode = !isLoginMode;
        document.getElementById('auth-title').textContent = isLoginMode ? 'Вход в систему' : 'Регистрация';
        document.getElementById('auth-submit-btn').textContent = isLoginMode ? 'Войти' : 'Создать аккаунт';
        document.getElementById('toggle-text').textContent = isLoginMode ? 'Ещё нет аккаунта?' : 'Уже есть аккаунт?';
        toggleBtn.textContent = isLoginMode ? 'Зарегистрироваться' : 'Войти';
    });

    authForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const email = document.getElementById('auth-email').value;
        const password = document.getElementById('auth-password').value;
        const errorEl = document.getElementById('auth-error');
        errorEl.style.display = 'none';

        try {
            if (isLoginMode) {
                await login(email, password);
            } else {
                await register(email, password);
            }
            checkSession(); // Переключаем экраны
        } catch (error) {
            errorEl.textContent = error.message || 'Ошибка аутентификации. Проверьте данные.';
            errorEl.style.display = 'block';
        }
    });

    // 2. Кнопка Выхода
    document.getElementById('logout-btn').addEventListener('click', () => {
        logout();
        checkSession();
    });

    // 3. Делегирование кликов по дереву задач
    const treeContainer = document.getElementById('tasks-tree-container');

    treeContainer.addEventListener('click', (e) => {
        const row = e.target.closest('.task-row');
        if (!row) return;

        // Если кликнули по чекбоксу — меняем статус задачи
        if (e.target.classList.contains('task-checkbox')) {
            e.stopPropagation();
            const id = e.target.dataset.id;
            const currentTask = state.tasks.get(id);
            const newStatus = e.target.checked ? 'done' : 'todo';

            // Отправляем изменения в PocketBase
            pb.collection('tasks').update(id, { status: newStatus });
            return;
        }

        // Иначе — открываем детальную карточку задачи
        const taskId = row.dataset.id;
        showDetails(taskId);
    });
}