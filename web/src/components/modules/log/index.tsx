'use client';

import { useCallback, useMemo } from 'react';
import { useLogs } from '@/api/endpoints/log';
import { LogCard } from './Item';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';

/**
 * 日志页面组件
 * - 初始加载 pageSize 条历史日志
 * - SSE 实时推送新日志
 * - 滚动自动加载更多
 */
export function Log() {
    const t = useTranslations('log');
    const { logs, hasMore, isLoading, isLoadingMore, loadMore } = useLogs({ pageSize: 20 });

    const canLoadMore = hasMore && !isLoading && !isLoadingMore && logs.length > 0;
    const handleReachEnd = useCallback(() => {
        if (!canLoadMore) return;
        void loadMore();
    }, [canLoadMore, loadMore]);

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

    // 骨架屏加载状态
    if (isLoading && logs.length === 0) {
        return (
            <div className="space-y-4 p-4">
                {Array.from({ length: 5 }).map((_, i) => (
                    <div key={i} className="rounded-3xl border bg-card p-4 animate-pulse">
                        <div className="flex items-center gap-4">
                            <div className="h-10 w-10 rounded-full bg-muted" />
                            <div className="flex-1 space-y-2">
                                <div className="h-4 w-3/4 rounded bg-muted" />
                                <div className="grid grid-cols-7 gap-4">
                                    {Array.from({ length: 7 }).map((_, j) => (
                                        <div key={j} className="h-3 rounded bg-muted" />
                                    ))}
                                </div>
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        );
    }

    return (
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
    );
}
