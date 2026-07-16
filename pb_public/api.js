export const pb = new PocketBase(window.location.origin);

// Проверка: есть ли сохраненная валидная сессия в LocalStorage
export function isAuthenticated() {
    return pb.authStore.isValid && pb.authStore.record !== null;
}

// Получить данные текущего пользователя
export function getCurrentUser() {
    return pb.authStore.record;
}

// Безопасный вход по данным пользователя
export async function login(email, password) {
    const authData = await pb.collection('users').authWithPassword(email, password);
    return authData.record;
}

// Безопасная регистрация нового пользователя
export async function register(email, password) {
    const data = {
        "email": email,
        "password": password,
        "passwordConfirm": password, // Для простоты формы подтверждение совпадает
        "emailVisibility": true
    };
    // Создаем пользователя в системной коллекции PocketBase
    await pb.collection('users').create(data);
    // Сразу авторизуем его после создания
    return await login(email, password);
}

// Выход из системы (стирает токен из локальной памяти браузера)
export function logout() {
    pb.authStore.clear();
}

// Ленивая загрузка деталей
export async function fetchTaskDetails(taskId) {
    try {
        const record = await pb.collection('task_details').getFirstListItem(`task_id="${taskId}"`);
        return {
            description: record.description_markdown || 'Описание отсутствует.',
            metaJson: record.meta_json || {},
            files: (record.attachments || []).map(f => ({ name: f, url: pb.files.getURL(record, f) }))
        };
    } catch (error) {
        if (error.status === 404) return { description: 'Нет детального описания.', metaJson: {}, files: [] };
        throw error;
    }
}
