package op

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/bestruirui/octopus/internal/db"
	"github.com/bestruirui/octopus/internal/model"
	"github.com/bestruirui/octopus/internal/utils/log"
	"github.com/bestruirui/octopus/internal/utils/snowflake"
	"gorm.io/gorm"
)

// LogSearchParams 日志搜索参数
type LogSearchParams struct {
	ModelName  string // 按模型名称搜索（LIKE 模糊匹配）
	APIKeyName string // 按 API Key 名称搜索（LIKE 模糊匹配）
	Keyword    string // 按错误信息关键词搜索（LIKE 模糊匹配）
}

// hasSearchFilters 检查是否有搜索过滤条件
func (p LogSearchParams) hasSearchFilters() bool {
	return p.ModelName != "" || p.APIKeyName != "" || p.Keyword != ""
}

// applySearchToQuery 将搜索条件应用到数据库查询
func applySearchToQuery(query *gorm.DB, search LogSearchParams) *gorm.DB {
	if search.ModelName != "" {
		query = query.Where("request_model_name LIKE ?", "%"+search.ModelName+"%")
	}
	if search.APIKeyName != "" {
		query = query.Where("request_api_key_name LIKE ?", "%"+search.APIKeyName+"%")
	}
	if search.Keyword != "" {
		query = query.Where("error LIKE ?", "%"+search.Keyword+"%")
	}
	return query
}

// matchSearchFilters 检查缓存的日志是否匹配搜索条件
func matchSearchFilters(log model.RelayLog, search LogSearchParams) bool {
	if search.ModelName != "" && !strings.Contains(log.RequestModelName, search.ModelName) {
		return false
	}
	if search.APIKeyName != "" && !strings.Contains(log.RequestAPIKeyName, search.APIKeyName) {
		return false
	}
	if search.Keyword != "" && !strings.Contains(log.Error, search.Keyword) {
		return false
	}
	return true
}

// relayLogListItemSelectColumns 查询列表时选择的列（排除大文本字段）
var relayLogListItemSelectColumns = []string{
	"id", "time", "request_model_name", "request_api_key_name",
	"channel_id", "channel_name", "actual_model_name",
	"input_tokens", "output_tokens", "cached_tokens",
	"ftut", "use_time", "cost", "error", "attempts", "total_attempts",
}

const relayLogMaxSize = 20
const relayLogMaxSizeNoDB = 100 // 当不保存到数据库时，允许更大的缓存用于实时查询

var relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
var relayLogCacheLock sync.Mutex

var relayLogFlushLock sync.Mutex

var relayLogSubscribers = make(map[chan model.RelayLog]struct{})
var relayLogSubscribersLock sync.RWMutex

var relayLogStreamTokens = make(map[string]struct{})
var relayLogStreamTokensLock sync.RWMutex

func RelayLogStreamTokenCreate() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	relayLogStreamTokensLock.Lock()
	relayLogStreamTokens[token] = struct{}{}
	relayLogStreamTokensLock.Unlock()

	return token, nil
}

func RelayLogStreamTokenVerify(token string) bool {
	relayLogStreamTokensLock.RLock()
	_, ok := relayLogStreamTokens[token]
	relayLogStreamTokensLock.RUnlock()
	return ok
}

func RelayLogStreamTokenRevoke(token string) {
	relayLogStreamTokensLock.Lock()
	delete(relayLogStreamTokens, token)
	relayLogStreamTokensLock.Unlock()
}

func RelayLogSubscribe() chan model.RelayLog {
	ch := make(chan model.RelayLog, 10)
	relayLogSubscribersLock.Lock()
	relayLogSubscribers[ch] = struct{}{}
	relayLogSubscribersLock.Unlock()
	return ch
}

func RelayLogUnsubscribe(ch chan model.RelayLog) {
	relayLogSubscribersLock.Lock()
	delete(relayLogSubscribers, ch)
	relayLogSubscribersLock.Unlock()
	close(ch)
}

func notifySubscribers(relayLog model.RelayLog) {
	relayLogSubscribersLock.RLock()
	defer relayLogSubscribersLock.RUnlock()

	for ch := range relayLogSubscribers {
		select {
		case ch <- relayLog:
		default:
		}
	}
}

func relayLogFlushToDB(ctx context.Context) error {
	relayLogFlushLock.Lock()
	defer relayLogFlushLock.Unlock()

	relayLogCacheLock.Lock()
	if len(relayLogCache) == 0 {
		relayLogCacheLock.Unlock()
		return nil
	}
	batch := make([]model.RelayLog, len(relayLogCache))
	copy(batch, relayLogCache)
	flushedUpto := len(batch)
	relayLogCacheLock.Unlock()

	result := db.GetDB().WithContext(ctx).Create(&batch)
	if result.Error != nil {
		return result.Error
	}

	relayLogCacheLock.Lock()
	if len(relayLogCache) >= flushedUpto {
		relayLogCache = relayLogCache[flushedUpto:]
	} else {
		relayLogCache = relayLogCache[:0]
	}
	if len(relayLogCache) == 0 {
		relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
	}
	relayLogCacheLock.Unlock()

	return nil
}

func RelayLogAdd(ctx context.Context, relayLog model.RelayLog) error {
	enabled, err := SettingGetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return err
	}
	maxSize := relayLogMaxSize
	if !enabled {
		maxSize = relayLogMaxSizeNoDB
	}
	relayLog.ID = snowflake.GenerateID()
	go notifySubscribers(relayLog)

	relayLogCacheLock.Lock()
	relayLogCache = append(relayLogCache, relayLog)
	if len(relayLogCache) >= maxSize {
		if enabled {
			relayLogCacheLock.Unlock()
			return relayLogFlushToDB(ctx)
		}
		// 如果未启用日志保存，移除最旧的日志，保留最新的日志用于实时查询
		keepSize := maxSize / 2
		if len(relayLogCache) > keepSize {
			relayLogCache = relayLogCache[len(relayLogCache)-keepSize:]
		}
	}
	relayLogCacheLock.Unlock()
	return nil
}

func RelayLogSaveDBTask(ctx context.Context) error {
	log.Debugf("relay log save db task started")
	startTime := time.Now()
	defer func() {
		log.Debugf("relay log save db task finished, save time: %s", time.Since(startTime))
	}()
	enabled, err := SettingGetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return err
	}

	if enabled {
		if err := relayLogFlushToDB(ctx); err != nil {
			return err
		}
		return relayLogCleanup(ctx)
	}

	// 如果未启用日志保存，检查缓存大小，如果超过限制则清理旧日志
	relayLogCacheLock.Lock()
	if len(relayLogCache) > relayLogMaxSizeNoDB {
		keepSize := relayLogMaxSizeNoDB / 2
		relayLogCache = relayLogCache[len(relayLogCache)-keepSize:]
	}
	relayLogCacheLock.Unlock()

	return nil
}

func relayLogCleanup(ctx context.Context) error {
	keepPeriod, err := SettingGetInt(model.SettingKeyRelayLogKeepPeriod)
	if err != nil {
		return err
	}

	if keepPeriod <= 0 {
		return nil
	}

	cutoffTime := time.Now().Add(-time.Duration(keepPeriod) * 24 * time.Hour).Unix()
	return db.GetDB().WithContext(ctx).Where("time < ?", cutoffTime).Delete(&model.RelayLog{}).Error
}

// RelayLogList 查询日志列表，支持可选的时间范围过滤
// startTime 和 endTime 为 nil 时表示不限制时间范围
func RelayLogList(ctx context.Context, startTime, endTime *int, page, pageSize int) ([]model.RelayLog, error) {
	enabled, err := SettingGetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}
	hasTimeFilter := startTime != nil && endTime != nil

	// 获取缓存中符合条件的日志
	relayLogCacheLock.Lock()
	var cachedLogs []model.RelayLog
	for _, log := range relayLogCache {
		if hasTimeFilter {
			if log.Time >= int64(*startTime) && log.Time <= int64(*endTime) {
				cachedLogs = append(cachedLogs, log)
			}
		} else {
			cachedLogs = append(cachedLogs, log)
		}
	}
	relayLogCacheLock.Unlock()

	// 反转缓存日志顺序（原本新的在末尾，反转后新的在前面，方便分页）
	for i, j := 0, len(cachedLogs)-1; i < j; i, j = i+1, j-1 {
		cachedLogs[i], cachedLogs[j] = cachedLogs[j], cachedLogs[i]
	}

	cacheCount := len(cachedLogs)
	offset := (page - 1) * pageSize

	var result []model.RelayLog

	// 先从缓存中取（缓存是最新的日志）
	if offset < cacheCount {
		cacheEnd := offset + pageSize
		if cacheEnd > cacheCount {
			cacheEnd = cacheCount
		}
		result = append(result, cachedLogs[offset:cacheEnd]...)
	}

	// 如果启用了日志保存，缓存不够时从数据库补充
	if enabled {
		remaining := pageSize - len(result)
		if remaining > 0 {
			dbOffset := 0
			if offset > cacheCount {
				dbOffset = offset - cacheCount
			}

			query := db.GetDB().WithContext(ctx)
			if hasTimeFilter {
				query = query.Where("time >= ? AND time <= ?", *startTime, *endTime)
			}

			var dbLogs []model.RelayLog
			if err := query.Order("id DESC").Offset(dbOffset).Limit(remaining).Find(&dbLogs).Error; err != nil {
				return nil, err
			}
			result = append(result, dbLogs...)
		}
	}

	return result, nil
}

// RelayLogListForAPI 查询日志列表（用于API返回，不包含大文本字段）
// 支持时间范围过滤和关键词搜索
func RelayLogListForAPI(ctx context.Context, startTime, endTime *int, search LogSearchParams, page, pageSize int) ([]model.RelayLogListItem, error) {
	enabled, err := SettingGetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}
	hasTimeFilter := startTime != nil && endTime != nil
	hasSearchFilter := search.hasSearchFilters()

	// 快速路径：无时间过滤、无搜索过滤、缓存足够的第一页数据可以直接从缓存返回
	if !hasTimeFilter && !hasSearchFilter && page == 1 {
		relayLogCacheLock.Lock()
		cacheCount := len(relayLogCache)
		if cacheCount >= pageSize {
			result := make([]model.RelayLogListItem, 0, pageSize)
			start := cacheCount - 1
			end := start - pageSize
			if end < -1 {
				end = -1
			}
			for i := start; i > end; i-- {
				result = append(result, convertToRelayLogListItem(relayLogCache[i]))
			}
			relayLogCacheLock.Unlock()
			return result, nil
		}
		relayLogCacheLock.Unlock()
	}

	// 有搜索过滤或无缓存时，直接从数据库查询（如果启用）
	if enabled && hasSearchFilter {
		offset := (page - 1) * pageSize
		query := db.GetDB().WithContext(ctx)
		if hasTimeFilter {
			query = query.Where("time >= ? AND time <= ?", *startTime, *endTime)
		}
		query = applySearchToQuery(query, search)

		var dbLogs []model.RelayLogListItem
		if err := query.Select(relayLogListItemSelectColumns).
			Order("id DESC").Offset(offset).Limit(pageSize).Find(&dbLogs).Error; err != nil {
			return nil, err
		}
		return dbLogs, nil
	}

	// 缓存为空且启用了数据库，直接查询数据库
	relayLogCacheLock.Lock()
	cacheCount := len(relayLogCache)
	relayLogCacheLock.Unlock()

	if cacheCount == 0 && enabled {
		offset := (page - 1) * pageSize
		query := db.GetDB().WithContext(ctx)
		if hasTimeFilter {
			query = query.Where("time >= ? AND time <= ?", *startTime, *endTime)
		}

		var dbLogs []model.RelayLogListItem
		if err := query.Select(relayLogListItemSelectColumns).
			Order("id DESC").Offset(offset).Limit(pageSize).Find(&dbLogs).Error; err != nil {
			return nil, err
		}
		return dbLogs, nil
	}

	// 通用处理逻辑：缓存 + 数据库混合
	offset := (page - 1) * pageSize
	var result []model.RelayLogListItem

	// 获取缓存中符合条件的日志
	relayLogCacheLock.Lock()
	var cachedLogs []model.RelayLog
	if hasTimeFilter || hasSearchFilter {
		for _, log := range relayLogCache {
			// 时间过滤
			if hasTimeFilter {
				if log.Time < int64(*startTime) || log.Time > int64(*endTime) {
					continue
				}
			}
			// 搜索过滤
			if hasSearchFilter && !matchSearchFilters(log, search) {
				continue
			}
			cachedLogs = append(cachedLogs, log)
		}
	} else {
		cachedLogs = make([]model.RelayLog, len(relayLogCache))
		copy(cachedLogs, relayLogCache)
	}
	relayLogCacheLock.Unlock()

	// 反转缓存日志顺序（原本新的在末尾，反转后新的在前面，方便分页）
	for i, j := 0, len(cachedLogs)-1; i < j; i, j = i+1, j-1 {
		cachedLogs[i], cachedLogs[j] = cachedLogs[j], cachedLogs[i]
	}

	cacheCount = len(cachedLogs)

	// 先从缓存中取
	if offset < cacheCount {
		cacheEnd := offset + pageSize
		if cacheEnd > cacheCount {
			cacheEnd = cacheCount
		}
		result = make([]model.RelayLogListItem, 0, cacheEnd-offset)
		for _, log := range cachedLogs[offset:cacheEnd] {
			result = append(result, convertToRelayLogListItem(log))
		}
	}

	// 从数据库补充
	if enabled {
		remaining := pageSize - len(result)
		if remaining > 0 {
			dbOffset := 0
			if offset > cacheCount {
				dbOffset = offset - cacheCount
			}

			query := db.GetDB().WithContext(ctx)
			if hasTimeFilter {
				query = query.Where("time >= ? AND time <= ?", *startTime, *endTime)
			}

			var dbLogs []model.RelayLogListItem
			if err := query.Select(relayLogListItemSelectColumns).
				Order("id DESC").Offset(dbOffset).Limit(remaining).Find(&dbLogs).Error; err != nil {
				return nil, err
			}
			result = append(result, dbLogs...)
		}
	}

	return result, nil
}

// convertToRelayLogListItem 将完整的RelayLog转换为列表项
func convertToRelayLogListItem(log model.RelayLog) model.RelayLogListItem {
	return model.RelayLogListItem{
		ID:                log.ID,
		Time:              log.Time,
		RequestModelName:  log.RequestModelName,
		RequestAPIKeyName: log.RequestAPIKeyName,
		ChannelId:         log.ChannelId,
		ChannelName:       log.ChannelName,
		ActualModelName:   log.ActualModelName,
		InputTokens:       log.InputTokens,
		OutputTokens:      log.OutputTokens,
		CachedTokens:      log.CachedTokens,
		Ftut:              log.Ftut,
		UseTime:           log.UseTime,
		Cost:              log.Cost,
		Error:             log.Error,
		Attempts:          log.Attempts,
		TotalAttempts:     log.TotalAttempts,
	}
}

// RelayLogGetByID 根据ID获取单个日志详情
func RelayLogGetByID(ctx context.Context, id int64) (*model.RelayLog, error) {
	// 先从缓存中查找
	relayLogCacheLock.Lock()
	for _, log := range relayLogCache {
		if log.ID == id {
			relayLogCacheLock.Unlock()
			return &log, nil
		}
	}
	relayLogCacheLock.Unlock()

	// 如果缓存中没有，从数据库查询
	var log model.RelayLog
	if err := db.GetDB().WithContext(ctx).Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}

	return &log, nil
}

func RelayLogClear(ctx context.Context) error {
	relayLogCacheLock.Lock()
	relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
	relayLogCacheLock.Unlock()
	return db.GetDB().WithContext(ctx).Where("1 = 1").Delete(&model.RelayLog{}).Error
}
