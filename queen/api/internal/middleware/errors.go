package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ─── Business Error Codes ───
// Format: module (2 digits) + sequence (3 digits)
// 00xxx = general, 01xxx = auth, 02xxx = billing, 03xxx = marketplace, 04xxx = bounty

const (
	// General
	CodeOK           = 0
	CodeBadRequest   = 400
	CodeUnauthorized = 401
	CodeForbidden    = 403
	CodeNotFound     = 404
	CodeConflict     = 409
	CodeRateLimited  = 429
	CodeInternal     = 500

	// Auth 01xxx
	CodeAuthInvalidCredentials = 1001
	CodeAuthTokenExpired       = 1002
	CodeAuthTokenInvalid       = 1003
	CodeAuthEmailTaken         = 1004
	CodeAuthPhoneTaken         = 1005
	CodeAuthUserNotFound       = 1006

	// Billing 02xxx
	CodeBillingInsufficient         = 2001
	CodeBillingOrderNotFound        = 2002
	CodeBillingPayNotConfigured     = 2003
	CodeBillingFreezeInsufficient   = 2004
	CodeBillingUnfreezeInsufficient = 2005

	// Marketplace 03xxx
	CodeMarketItemNotFound  = 3001
	CodeMarketItemOwnership = 3002

	// Bounty 04xxx
	CodeBountyNotFound       = 4001
	CodeBountyStatusConflict = 4002
	CodeBountyPermission     = 4003

	// Credits 05xxx
	CodeCreditInsufficient     = 5001
	CodeCreditInvalidSignature = 5002
	CodeCreditNonceExpired     = 5003
	CodeCreditAccountNotFound  = 5004
	CodeCreditHibernated       = 5005
)

// ErrMsg maps error codes to default Chinese messages
var ErrMsg = map[int]string{
	CodeOK:           "成功",
	CodeBadRequest:   "请求参数错误",
	CodeUnauthorized: "未登录",
	CodeForbidden:    "权限不足",
	CodeNotFound:     "资源不存在",
	CodeConflict:     "状态冲突",
	CodeRateLimited:  "请求过于频繁，请稍后再试",
	CodeInternal:     "服务器内部错误",

	CodeAuthInvalidCredentials: "用户名或密码错误",
	CodeAuthTokenExpired:       "登录已过期，请重新登录",
	CodeAuthTokenInvalid:       "无效的登录凭证",
	CodeAuthEmailTaken:         "该邮箱已被注册",
	CodeAuthPhoneTaken:         "该手机号已被注册",
	CodeAuthUserNotFound:       "用户不存在",

	CodeBillingInsufficient:         "余额不足",
	CodeBillingOrderNotFound:        "订单不存在",
	CodeBillingPayNotConfigured:     "支付方式尚未配置",
	CodeBillingFreezeInsufficient:   "可用余额不足，无法冻结",
	CodeBillingUnfreezeInsufficient: "冻结余额不足，无法解冻",

	CodeMarketItemNotFound:  "市场商品不存在",
	CodeMarketItemOwnership: "无权操作此商品",

	CodeBountyNotFound:       "赏金任务不存在",
	CodeBountyStatusConflict: "赏金任务状态不允许此操作",
	CodeBountyPermission:     "无权操作此赏金任务",

	CodeCreditInsufficient:     "星力余额不足",
	CodeCreditInvalidSignature: "签名验证失败",
	CodeCreditNonceExpired:     "nonce 过期",
	CodeCreditAccountNotFound:  "星力账户不存在",
	CodeCreditHibernated:       "节点已休眠（星力耗尽）",
}

// APIResponse is the standardized JSON envelope
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OK sends a success response
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    CodeOK,
		Message: "成功",
		Data:    data,
	})
}

// Fail sends an error response with the given business error code
func Fail(c *gin.Context, httpStatus int, code int, msgOverride ...string) {
	msg := ErrMsg[code]
	if len(msgOverride) > 0 && msgOverride[0] != "" {
		msg = msgOverride[0]
	}
	if msg == "" {
		msg = "未知错误"
	}
	c.JSON(httpStatus, APIResponse{
		Code:    code,
		Message: msg,
	})
}

// FailWithData sends an error response with additional data
func FailWithData(c *gin.Context, httpStatus int, code int, data interface{}, msgOverride ...string) {
	msg := ErrMsg[code]
	if len(msgOverride) > 0 && msgOverride[0] != "" {
		msg = msgOverride[0]
	}
	c.JSON(httpStatus, APIResponse{
		Code:    code,
		Message: msg,
		Data:    data,
	})
}
