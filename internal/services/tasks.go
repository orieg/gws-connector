package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
)

// defaultTasklist is the Google Tasks alias for the account's default list.
const defaultTasklist = "@default"

// TasksService implements Google Tasks MCP tools.
type TasksService struct {
	router  *accounts.Router
	clients *auth.ClientFactory
}

// NewTasksService creates a new tasks service.
func NewTasksService(router *accounts.Router, clients *auth.ClientFactory) *TasksService {
	return &TasksService{router: router, clients: clients}
}

func (t *TasksService) resolveAndGetService(ctx context.Context, args map[string]any) (*tasks.Service, *accounts.Account, error) {
	accountParam, _ := args["account"].(string)
	acct, err := t.router.Resolve(accountParam)
	if err != nil {
		return nil, nil, err
	}
	svc, err := t.clients.TasksService(ctx, acct.Email)
	if err != nil {
		return nil, nil, err
	}
	return svc, acct, nil
}

// tasklistArg returns the tasklist argument, defaulting to "@default".
func tasklistArg(args map[string]any) string {
	if tl, ok := args["tasklist"].(string); ok && tl != "" {
		return tl
	}
	return defaultTasklist
}

// ListTasklists lists the account's task lists.
func (t *TasksService) ListTasklists(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := t.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	resp, err := svc.Tasklists.List().Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Tasks", err, "listing task lists on %s: %w", acct.Label, err)), nil
	}

	if len(resp.Items) == 0 {
		return TextResult(fmt.Sprintf("No task lists found on %s (%s).", acct.Label, acct.Email)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task lists on %s (%s) — %d found:\n\n", acct.Label, acct.Email, len(resp.Items)))
	for i, tl := range resp.Items {
		sb.WriteString(fmt.Sprintf("%d. %s\n   ID: %s\n\n", i+1, tl.Title, tl.Id))
	}
	return TextResult(sb.String()), nil
}

// ListTasks lists the tasks in a task list.
func (t *TasksService) ListTasks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	svc, acct, err := t.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	tasklist := tasklistArg(args)
	showCompleted, _ := args["showCompleted"].(bool)

	call := svc.Tasks.List(tasklist).ShowCompleted(showCompleted)
	// ShowCompleted is only honored when ShowHidden is also set — completed
	// tasks become hidden once the list is cleared.
	if showCompleted {
		call = call.ShowHidden(true)
	}

	resp, err := call.Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Tasks", err, "listing tasks on %s: %w", acct.Label, err)), nil
	}

	if len(resp.Items) == 0 {
		return TextResult(fmt.Sprintf("No tasks found in list %q on %s (%s).", tasklist, acct.Label, acct.Email)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tasks in list %q on %s (%s) — %d found:\n\n", tasklist, acct.Label, acct.Email, len(resp.Items)))
	for i, task := range resp.Items {
		status := task.Status
		if status == "completed" {
			status = "completed ✓"
		}
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s]\n", i+1, task.Title, status))
		if task.Due != "" {
			sb.WriteString(fmt.Sprintf("   Due: %s\n", task.Due))
		}
		if task.Notes != "" {
			notes := task.Notes
			if len(notes) > 200 {
				notes = notes[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("   Notes: %s\n", notes))
		}
		sb.WriteString(fmt.Sprintf("   ID: %s\n\n", task.Id))
	}
	return TextResult(sb.String()), nil
}

// Create adds a new task to a task list.
func (t *TasksService) Create(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	title, _ := args["title"].(string)
	if title == "" {
		return ErrorResult(fmt.Errorf("title is required")), nil
	}

	svc, acct, err := t.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	tasklist := tasklistArg(args)
	notes, _ := args["notes"].(string)
	due, _ := args["due"].(string)

	task := &tasks.Task{
		Title: title,
		Notes: notes,
		Due:   due,
	}

	created, err := svc.Tasks.Insert(tasklist, task).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Tasks", err, "creating task on %s: %w", acct.Label, err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task created on %s (%s):\n", acct.Label, acct.Email))
	sb.WriteString(fmt.Sprintf("  Title: %s\n", created.Title))
	if created.Due != "" {
		sb.WriteString(fmt.Sprintf("  Due: %s\n", created.Due))
	}
	sb.WriteString(fmt.Sprintf("  List: %s\n", tasklist))
	sb.WriteString(fmt.Sprintf("  ID: %s\n", created.Id))
	return TextResult(sb.String()), nil
}

// Complete marks a task as completed.
func (t *TasksService) Complete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	taskId, _ := args["taskId"].(string)
	if taskId == "" {
		return ErrorResult(fmt.Errorf("taskId is required")), nil
	}

	svc, acct, err := t.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	tasklist := tasklistArg(args)

	updated, err := svc.Tasks.Patch(tasklist, taskId, &tasks.Task{Status: "completed"}).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Tasks", err, "completing task on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Task completed on %s (%s):\n  Title: %s\n  Status: %s\n  List: %s\n  ID: %s",
		acct.Label, acct.Email, updated.Title, updated.Status, tasklist, updated.Id,
	)), nil
}

// Delete permanently removes a task from a task list.
func (t *TasksService) Delete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	taskId, _ := args["taskId"].(string)
	if taskId == "" {
		return ErrorResult(fmt.Errorf("taskId is required")), nil
	}

	svc, acct, err := t.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	tasklist := tasklistArg(args)

	if err := svc.Tasks.Delete(tasklist, taskId).Do(); err != nil {
		return ErrorResult(scopeOrErr(acct, "Tasks", err, "deleting task on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Task deleted on %s (%s):\n  List: %s\n  ID: %s",
		acct.Label, acct.Email, tasklist, taskId,
	)), nil
}
