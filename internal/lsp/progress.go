package lsp

const workspaceIndexProgressToken = "krit/workspace-index"

type WorkDoneProgressParams struct {
	Token string                `json:"token"`
	Value WorkDoneProgressValue `json:"value"`
}

type WorkDoneProgressValue struct {
	Kind       string `json:"kind"`
	Title      string `json:"title,omitempty"`
	Message    string `json:"message,omitempty"`
	Percentage int    `json:"percentage,omitempty"`
}

func (s *Server) reportWorkspaceIndexBegin() {
	if !s.workDoneProgress {
		return
	}
	s.sendNotification("$/progress", WorkDoneProgressParams{
		Token: workspaceIndexProgressToken,
		Value: WorkDoneProgressValue{
			Kind:  "begin",
			Title: "Krit workspace index",
		},
	})
}

func (s *Server) reportWorkspaceIndexProgress(done, total int) {
	if !s.workDoneProgress {
		return
	}
	if total <= 0 {
		return
	}
	percentage := done * 100 / total
	s.sendNotification("$/progress", WorkDoneProgressParams{
		Token: workspaceIndexProgressToken,
		Value: WorkDoneProgressValue{
			Kind:       "report",
			Message:    "Indexing workspace",
			Percentage: percentage,
		},
	})
}

func (s *Server) reportWorkspaceIndexEnd(message string) {
	if !s.workDoneProgress {
		return
	}
	s.sendNotification("$/progress", WorkDoneProgressParams{
		Token: workspaceIndexProgressToken,
		Value: WorkDoneProgressValue{
			Kind:    "end",
			Message: message,
		},
	})
}
