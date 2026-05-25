package client

import (
	"context"
	dsprotocol "ds2api/internal/deepseek/protocol"

	"ds2api/internal/config"
)

// GetMuteStatus 查询账号禁言状态，调用 GET /api/v0/users/current
// 返回 config.MuteInfo，Muted=true 表示被禁言，MuteUntil 为秒级禁言解除时间戳
func (c *Client) GetMuteStatus(ctx context.Context, token string) (*config.MuteInfo, error) {
	clients := c.requestClientsFromContext(ctx)
	headers := c.authHeaders(token)

	resp, status, err := c.getJSONWithStatus(ctx, clients.regular, dsprotocol.DeepSeekUserCurrentURL, headers)
	if err != nil {
		return nil, err
	}

	if status != 200 {
		return nil, nil
	}

	code := intFrom(resp["code"])
	if code != 0 {
		return nil, nil
	}

	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return nil, nil
	}

	bizCode := intFrom(data["biz_code"])
	if bizCode != 0 {
		return nil, nil
	}

	bizData, _ := data["biz_data"].(map[string]any)
	if bizData == nil {
		return nil, nil
	}

	chat, _ := bizData["chat"].(map[string]any)
	if chat == nil {
		return nil, nil
	}

	isMuted, _ := chat["is_muted"].(float64)
	muteUntil, _ := chat["mute_until"].(float64)

	return &config.MuteInfo{
		Muted:     isMuted == 1,
		MuteUntil: int64(muteUntil),
	}, nil
}
