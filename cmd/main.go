package main

import (
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/ghupdate"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	_ "task-orchestrator/internal/migrations"
)

func main() {
	app := pocketbase.New()

	// 1. Регистрируем плагины (из пакетов plugins/...) в соответствии с вашим кодом
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: true, // Автоматически генерирует новые файлы миграций при изменении через UI
	})
	ghupdate.MustRegister(app, app.RootCmd, ghupdate.Config{})

	// 2. Логика автоматической цепочки задач при обновлении базы данных
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {

		e.App.OnRecordUpdate().BindFunc(func(re *core.RecordEvent) error {
			// Работаем только с коллекцией "tasks"
			if re.Record.Collection().Name != "tasks" {
				return re.Next()
			}

			// Если ИИ перевел задачу в статус 'done'
			if re.Record.GetString("status") != "done" {
				return re.Next()
			}

			taskID := re.Record.Id
			projectID := re.Record.GetString("project_id")

			// Ищем правило в workflow_rules для этой задачи
			rules, err := re.App.FindRecordsByFilter(
				"workflow_rules",
				"trigger_task_id = {:taskID} && trigger_status = 'done'",
				"-created",
				1, 0,
				map[string]any{"taskID": taskID},
			)

			if err != nil || len(rules) == 0 {
				return re.Next() // Правил нет, идем дальше
			}

			rule := rules[0]

			if rule.GetString("action_type") == "create_next_task" {
				tasksCollection, err := re.App.FindCollectionByNameOrId("tasks")
				if err != nil {
					return err
				}

				// Запускаем транзакцию для безопасности данных
				err = re.App.RunInTransaction(func(txApp core.App) error {
					nextTask := core.NewRecord(tasksCollection)
					nextTask.Set("project_id", projectID)
					nextTask.Set("parent_id", taskID) // Указываем завершенную задачу как родительскую
					nextTask.Set("title", rule.GetString("next_title"))
					nextTask.Set("status", "todo")

					assigneeType := rule.GetString("next_assignee_type")
					nextTask.Set("assignee_type", assigneeType)

					if assigneeType == "human" {
						nextTask.Set("assigned_user_id", rule.GetString("next_assigned_user_id"))
					} else if assigneeType == "ai" {
						nextTask.Set("assigned_agent_id", rule.GetString("next_assigned_agent_id"))
					}

					return txApp.Save(nextTask)
				})

				if err != nil {
					log.Printf("[Workflow Error] Ошибка создания задачи: %v", err)
					return err
				}
			}

			return re.Next()
		})

		return e.Next()
	})

	// 3. Конфигурация веб-сервера (Роутинг, CORS и раздача Статики)
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {

		// Настройка CORS глобально через .Bind на корневой роутер
		se.Router.Bind(apis.CORS(apis.CORSConfig{
			AllowOrigins: []string{
				"https://astro3d.ru",
				"http://10.66.66.9:8090",
				"http://localhost:8090",
			},
			AllowHeaders: []string{"Content-Type", "Authorization"},
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		}))

		// Раздача статических файлов из директории pb_public (HTML, Vanilla JS)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), true))

		return se.Next()
	})

	// Запуск приложения
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
