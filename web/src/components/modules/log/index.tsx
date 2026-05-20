'use client';

import { useCallback, useMemo, useState, useRef } from 'react';
import { useLogs, type LogSearchFilters } from '@/api/endpoints/log';
import { LogCard } from './Item';
import { Loader2, Search, X } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { motion, AnimatePresence } from 'motion/react';

/**
 * 骨架屏组件
 */
function LogSkeleton({ count = 6 }: { count?: number }) {
    return (
        <div className="space-y-4 p-4">
            {Array.from({ length: count }).map((_, i) => (
                <motion.div
                    key={i}
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: i * 0.05, duration: 0.3 }}
                    className="rounded-3xl border bg-card p-4"
                >
                    <div className="flex items-center gap-4">
                        <div className="h-10 w-10 rounded-full bg-muted animate-pulse" />
                        <div className="flex-1 space-y-3">
                            <div className="flex items-center gap-2">
                                <div className="h-4 w-32 rounded bg-muted animate-pulse" />
                                <div className="h-4 w-4 rounded bg-muted animate-pulse" />
                                <div className="h-5 w-20 rounded-full bg-muted animate-pulse" />
                                <div className="h-4 w-24 rounded bg-muted animate-pulse" />
                            </div>
                            <div className="grid grid-cols-7 gap-3">
                                {Array.from({ length: 7 }).map((_, j) => (
                                    <div key={j} className="flex items-center gap-1.5">
                                        <div className="h-3.5 w-3.5 rounded bg-muted animate-pulse" />
                                        <div className="h-3 flex-1 rounded bg-muted animate-pulse" />
                                    </div>
                                ))}
                            </div>
                        </div>
                    </div>
                </motion.div>
            ))}
        </div>
    );
}

/**
 * 日志搜索栏
 */
function LogSearchBar({ onSearch }: { onSearch: (filters: LogSearchFilters) => void }) {
    const t = useTranslations('log');
    const [modelName, setModelName] = useState('');
    const [apiKeyName, setApiKeyName] = useState('');
    const [keyword, setKeyword] = useState('');
    const timerRef = useRef<ReturnType<typeof setTimeout>>(null);

    const doSearch = useCallback(() => {
        if (timerRef.current) clearTimeout(timerRef.current);
        timerRef.current = setTimeout(() => {
            onSearch({
                modelName: modelName.trim() || undefined,
                apiKeyName: apiKeyName.trim() || undefined,
                keyword: keyword.trim() || undefined,
            });
        }, 300);
    }, [modelName, apiKeyName, keyword, onSearch]);

    const clearAll = useCallback(() => {
        setModelName('');
        setApiKeyName('');
        setKeyword('');
        onSearch({});
    }, [onSearch]);

    const hasFilters = modelName || apiKeyName || keyword;

    return (
        <div className="flex flex-wrap items-center gap-2 p-3 bg-card rounded-2xl border border-border">
            <Search className="size-4 text-muted-foreground shrink-0" />
            <input
                type="text"
                placeholder={t('search.modelName')}
                value={modelName}
                onChange={(e) => { setModelName(e.target.value); doSearch(); }}
                className="flex-1 min-w-[120px] bg-transparent border-none outline-none text-sm placeholder:text-muted-foreground/50"
            />
            <input
                type="text"
                placeholder={t('search.apiKeyName')}
                value={apiKeyName}
                onChange={(e) => { setApiKeyName(e.target.value); doSearch(); }}
                className="flex-1 min-w-[120px] bg-transparent border-none outline-none text-sm placeholder:text-muted-foreground/50"
            />
            <input
                type="text"
                placeholder={t('search.keyword')}
                value={keyword}
                onChange={(e) => { setKeyword(e.target.value); doSearch(); }}
                className="flex-1 min-w-[100px] bg-transparent border-none outline-none text-sm placeholder:text-muted-foreground/50"
            />
            {hasFilters && (
                <button
                    onClick={clearAll}
                    className="shrink-0 p-1 rounded-lg hover:bg-muted transition-colors"
                >
                    <X className="size-4 text-muted-foreground" />
                </button>
            )}
        </div>
    );
}

/**
 * 日志页面组件
 * - 初始加载 pageSize 条历史日志
 * - SSE 实时推送新日志
 * - 滚动自动加载更多
 * - 搜索过滤
 */
export function Log() {
    const t = useTranslations('log');
    const [searchFilters, setSearchFilters] = useState<LogSearchFilters>({});
    const { logs, hasMore, isLoading, isLoadingMore, loadMore } = useLogs({ pageSize: 20, search: searchFilters });

    const canLoadMore = hasMore && !isLoading && !isLoadingMore && logs.length > 0;
    const handleReachEnd = useCallback(() => {
        if (!canLoadMore) return;
        void loadMore();
    }, [canLoadMore, loadMore]);

    const handleSearch = useCallback((filters: LogSearchFilters) => {
        setSearchFilters(filters);
    }, []);

    const footer = useMemo(() => {
        if (hasMore && (isLoading || isLoadingMore)) {
            return (
                <div className="flex justify-center py-4">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
            );
        }
        if (!hasMore && logs.length > 0) {
            return (
                <div className="flex justify-center py-4">
                    <span className="text-sm text-muted-foreground">{t('list.noMore')}</span>
                </div>
            );
        }
        return null;
    }, [hasMore, isLoading, isLoadingMore, logs.length, t]);

    // 加载状态 - 显示骨架屏
    if (isLoading && logs.length === 0) {
        return (
            <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                className="h-full flex flex-col gap-4"
            >
                <LogSearchBar onSearch={handleSearch} />
                <LogSkeleton count={6} />
            </motion.div>
        );
    }

    // 空状态
    if (!isLoading && logs.length === 0) {
        return (
            <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                className="h-full flex flex-col gap-4"
            >
                <LogSearchBar onSearch={handleSearch} />
                <motion.div
                    initial={{ opacity: 0, scale: 0.95 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ duration: 0.3 }}
                    className="flex flex-col items-center justify-center h-64 gap-4"
                >
                    <div className="h-16 w-16 rounded-full bg-muted flex items-center justify-center">
                        <svg
                            xmlns="http://www.w3.org/2000/svg"
                            width="32"
                            height="32"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="1.5"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            className="text-muted-foreground"
                        >
                            <path d="M12 20h9" />
                            <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
                        </svg>
                    </div>
                    <p className="text-muted-foreground text-sm">{t('list.empty')}</p>
                </motion.div>
            </motion.div>
        );
    }

    return (
        <AnimatePresence mode="wait">
            <motion.div
                key="log-list"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.2 }}
                className="h-full flex flex-col gap-2"
            >
                <LogSearchBar onSearch={handleSearch} />
                <div className="flex-1 min-h-0">
                    <VirtualizedGrid
                        items={logs}
                        layout="list"
                        columns={{ default: 1 }}
                        estimateItemHeight={80}
                        overscan={4}
                        getItemKey={(log) => `log-${log.id}`}
                        renderItem={(log) => <LogCard log={log} />}
                        footer={footer}
                        onReachEnd={handleReachEnd}
                        reachEndEnabled={canLoadMore}
                        reachEndOffset={2}
                    />
                </div>
            </motion.div>
        </AnimatePresence>
    );
}
