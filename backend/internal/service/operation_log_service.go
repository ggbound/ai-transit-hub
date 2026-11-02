package service

import (
	"encoding/json"

	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// OperationLogService 负责操作审计日志的落库。
// 记录失败仅打日志，绝不影响主业务流程。
type OperationLogService struct{}

// Operation 包级单例，调用点形如 service.Operation.Record(...)
var Operation = &OperationLogService{}

// OperationLogEntry 操作日志入参
type OperationLogEntry struct {
	UserID    *uint  // 操作人ID（未知时为 nil，如登录失败）
	Username  string // 操作人姓名（冗余存储，便于检索展示）
	Type      string // 操作类型：login/logout/create/update/toggle/delete
	Module    string // 操作模块：auth/users/models/billing...
	Action    string // 操作描述：如 "POST /login"、"PUT /users/1/toggle-status"
	Params    any    // 请求参数（自动 JSON 编码，超长截断）
	Result    any    // 响应结果（自动 JSON 编码，超长截断）
	Status    string // success / failed，空串默认 success
	ErrorMsg  string // 失败原因
	IP        string
	UserAgent string
}

// maxOperationLogJSON 操作日志 JSON 字段落库字节上限（text/json 列安全值）
const maxOperationLogJSON = 16384

// Record 异步写入一条操作日志
func (s *OperationLogService) Record(entry OperationLogEntry) {
	// 先在请求 goroutine 内完成 JSON 编码（gin context 不可跨 goroutine 读取）
	log := models.OperationLog{
		Username:        entry.Username,
		OperationType:   entry.Type,
		OperationModule: entry.Module,
		OperationAction: entry.Action,
		IpAddress:       entry.IP,
		UserAgent:       entry.UserAgent,
		UserID:          entry.UserID,
		Status:          entry.Status,
	}

	if log.Status == "" {
		log.Status = "success"
	}
	if entry.ErrorMsg != "" {
		msg := TruncateUTF8(entry.ErrorMsg, 1024)
		log.ErrorMessage = &msg
	}

	if entry.Params != nil {
		if b, err := json.Marshal(entry.Params); err == nil {
			log.RequestParams = models.JSON(TruncateUTF8(string(b), maxOperationLogJSON))
		}
	}
	if entry.Result != nil {
		if b, err := json.Marshal(entry.Result); err == nil {
			log.ResponseResult = models.JSON(TruncateUTF8(string(b), maxOperationLogJSON))
		}
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("operation log panicked: %v", r)
			}
		}()
		if err := repository.DB.Create(&log).Error; err != nil {
			logger.Error("Failed to create operation log: %v", err)
		}
	}()
}
