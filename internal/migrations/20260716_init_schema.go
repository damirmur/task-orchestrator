// internal/migrations/20260716_init_schema.go
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		rule := "@request.auth.id != ''"

		// === ИДЕНТИФИКАТОРЫ КОЛЛЕКЦИЙ (СТРОГО 15 СИМВОЛОВ, МАЛЕНЬКИЕ БУКВЫ + ЦИФРЫ) ===
		const (
			projectsID = "pbcprojects1111"
			agentsID   = "pbcaiagents1111"
			tasksID    = "pbctaskscore111"
			detailsID  = "pbctaskdetails1"
			workflowID = "pbcworkflows111"
		)

		// === ЭТАП 1: СОЗДАНИЕ КОЛЛЕКЦИЙ ===

		projects := core.NewBaseCollection("projects")
		projects.Id = projectsID
		projects.ViewRule = &rule
		projects.CreateRule = &rule
		projects.UpdateRule = &rule
		projects.DeleteRule = &rule
		projects.Fields.Add(&core.TextField{Name: "name", Required: true})
		if err := app.Save(projects); err != nil {
			return err
		}

		agents := core.NewBaseCollection("ai_agents_config")
		agents.Id = agentsID
		agents.ViewRule = &rule
		agents.CreateRule = &rule
		agents.UpdateRule = &rule
		agents.DeleteRule = &rule
		agents.Fields.Add(&core.TextField{Name: "name", Required: true})
		agents.Fields.Add(&core.SelectField{Name: "provider", Values: []string{"openai", "anthropic", "ollama_local", "vllm"}})
		agents.Fields.Add(&core.TextField{Name: "model_name", Required: true})
		agents.Fields.Add(&core.TextField{Name: "system_prompt"})
		if err := app.Save(agents); err != nil {
			return err
		}

		tasks := core.NewBaseCollection("tasks")
		tasks.Id = tasksID
		tasks.ViewRule = &rule
		tasks.CreateRule = &rule
		tasks.UpdateRule = &rule
		tasks.DeleteRule = &rule
		tasks.Fields.Add(&core.TextField{Name: "title", Required: true})
		tasks.Fields.Add(&core.SelectField{Name: "status", Values: []string{"backlog", "todo", "in_progress", "done", "failed"}})
		tasks.Fields.Add(&core.SelectField{Name: "assignee_type", Values: []string{"human", "ai"}})
		if err := app.Save(tasks); err != nil {
			return err
		}

		details := core.NewBaseCollection("task_details")
		details.Id = detailsID
		details.ViewRule = &rule
		details.CreateRule = &rule
		details.UpdateRule = &rule
		details.DeleteRule = &rule
		details.Fields.Add(&core.TextField{Name: "description_markdown"})
		details.Fields.Add(&core.JSONField{Name: "meta_json"})
		details.Fields.Add(&core.FileField{Name: "attachments", MaxSelect: 20, MaxSize: 52428800})
		if err := app.Save(details); err != nil {
			return err
		}

		workflow := core.NewBaseCollection("workflow_rules")
		workflow.Id = workflowID
		workflow.ViewRule = &rule
		workflow.CreateRule = &rule
		workflow.UpdateRule = &rule
		workflow.DeleteRule = &rule
		workflow.Fields.Add(&core.SelectField{Name: "trigger_status", Values: []string{"done", "failed"}})
		workflow.Fields.Add(&core.SelectField{Name: "action_type", Values: []string{"create_next_task"}})
		workflow.Fields.Add(&core.TextField{Name: "next_title", Required: true})
		workflow.Fields.Add(&core.SelectField{Name: "next_assignee_type", Values: []string{"human", "ai"}})
		if err := app.Save(workflow); err != nil {
			return err
		}

		// === ЭТАП 2: НАСТРОЙКА СВЯЗЕЙ ===

		projects.Fields.Add(&core.RelationField{Name: "members", CollectionId: "_pb_users_auth_", MaxSelect: 999})
		if err := app.Save(projects); err != nil {
			return err
		}

		tasks.Fields.Add(&core.RelationField{Name: "project_id", CollectionId: projectsID, Required: true, MaxSelect: 1, CascadeDelete: true})
		tasks.Fields.Add(&core.RelationField{Name: "parent_id", CollectionId: tasksID, MaxSelect: 1, CascadeDelete: true})
		tasks.Fields.Add(&core.RelationField{Name: "assigned_user_id", CollectionId: "_pb_users_auth_", MaxSelect: 1})
		tasks.Fields.Add(&core.RelationField{Name: "assigned_agent_id", CollectionId: agentsID, MaxSelect: 1})
		if err := app.Save(tasks); err != nil {
			return err
		}

		details.Fields.Add(&core.RelationField{Name: "task_id", CollectionId: tasksID, Required: true, MaxSelect: 1, CascadeDelete: true})
		if err := app.Save(details); err != nil {
			return err
		}

		workflow.Fields.Add(&core.RelationField{Name: "project_id", CollectionId: projectsID, Required: true, MaxSelect: 1, CascadeDelete: true})
		workflow.Fields.Add(&core.RelationField{Name: "trigger_task_id", CollectionId: tasksID, Required: true, MaxSelect: 1, CascadeDelete: true})
		workflow.Fields.Add(&core.RelationField{Name: "next_assigned_user_id", CollectionId: "_pb_users_auth_", MaxSelect: 1})
		workflow.Fields.Add(&core.RelationField{Name: "next_assigned_agent_id", CollectionId: agentsID, MaxSelect: 1})
		if err := app.Save(workflow); err != nil {
			return err
		}

		// === ЭТАП 3: НАПОЛНЕНИЕ ДАННЫМИ (СТРОГО 15 СИМВОЛОВ НА КАЖДЫЙ ID ЗАПИСИ) ===
		const (
			testProjID   = "projecttest0001"
			testRootTask = "taskroot0000001"
			testAiTask   = "taskai000000001"
			testDetailID = "detailtest00001"
			testRuleID   = "ruletest0000001"
		)

		projRecord := core.NewRecord(projects)
		projRecord.Id = testProjID
		projRecord.Set("name", "Мой Первый ИИ Проект")
		if err := app.Save(projRecord); err != nil {
			return err
		}

		rootTask := core.NewRecord(tasks)
		rootTask.Id = testRootTask
		rootTask.Set("project_id", testProjID)
		rootTask.Set("title", "Родительская задача (Анализ рынка)")
		rootTask.Set("status", "in_progress")
		rootTask.Set("assignee_type", "human")
		if err := app.Save(rootTask); err != nil {
			return err
		}

		aiTask := core.NewRecord(tasks)
		aiTask.Id = testAiTask
		aiTask.Set("project_id", testProjID)
		aiTask.Set("parent_id", testRootTask)
		aiTask.Set("title", "Подзадача для Робота: Собрать данные")
		aiTask.Set("status", "todo")
		aiTask.Set("assignee_type", "ai")
		if err := app.Save(aiTask); err != nil {
			return err
		}

		taskDetail := core.NewRecord(details)
		taskDetail.Id = testDetailID
		taskDetail.Set("task_id", testAiTask)
		taskDetail.Set("description_markdown", "Нужно собрать топ-10 трендов из интернета")
		taskDetail.Set("meta_json", map[string]any{"max_tokens": 2000, "temperature": 0.2})
		if err := app.Save(taskDetail); err != nil {
			return err
		}

		ruleRecord := core.NewRecord(workflow)
		ruleRecord.Id = testRuleID
		ruleRecord.Set("project_id", testProjID)
		ruleRecord.Set("trigger_task_id", testAiTask)
		ruleRecord.Set("trigger_status", "done")
		ruleRecord.Set("action_type", "create_next_task")
		ruleRecord.Set("next_title", "Проверить отчет, сгенерированный нейросетью")
		ruleRecord.Set("next_assignee_type", "human")
		if err := app.Save(ruleRecord); err != nil {
			return err
		}

		return nil
	}, nil)
}

// Вспомогательная функция регистрации мета-данных коллекций для UI PocketBase
func insertMetaCollections(app core.App) error {
	tables := []string{"projects", "tasks", "task_details", "ai_agents_config", "workflow_rules"}

	for _, tableName := range tables {
		var count int
		err := app.DB().NewQuery("SELECT count(*) FROM [[_collections]] WHERE [[name]] = {:name}").
			Bind(map[string]any{"name": tableName}).
			Row(&count)

		if err == nil && count == 0 {
			_, err = app.DB().NewQuery(`
				INSERT INTO [[_collections]] ([[id]], [[name]], [[type]], [[system]], [[viewRule]], [[createRule]], [[updateRule]], [[deleteRule]], [[fields]])
				VALUES (
					{:id}, {:name}, 'base', 0, 
					'@request.auth.id != ""', '@request.auth.id != ""', 
					'@request.auth.id != ""', '@request.auth.id != ""', 
					'[]'
				);
			`).Bind(map[string]any{
				"id":   tableName,
				"name": tableName,
			}).Execute()

			if err != nil {
				return err
			}
		}
	}
	return nil
}
