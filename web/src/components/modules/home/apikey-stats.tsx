'use client';

import { useStatsAPIKey } from '@/api/endpoints/stats';
import { useAPIKeyList, type APIKey } from '@/api/endpoints/apikey';
import { motion } from 'motion/react';
import { KeyRound, DollarSign, MessageSquare, Zap, AlertTriangle } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useMemo } from 'react';
import { EASING } from '@/lib/animations/fluid-transitions';
import { AnimatedNumber } from '@/components/common/AnimatedNumber';
import { Progress } from '@/components/ui/progress';

/**
 * 管理员视角：所有 API Key 的用量统计
 */
export function APIKeyStatsPanel() {
    const t = useTranslations('home.apiKeyStats');
    const { data: apiKeyStats } = useStatsAPIKey();
    const { data: apiKeys } = useAPIKeyList();

    // 构建 ID -> Name 映射
    const nameMap = useMemo(() => {
        const map = new Map<number, APIKey>();
        if (apiKeys) {
            for (const key of apiKeys) {
                map.set(key.id, key);
            }
        }
        return map;
    }, [apiKeys]);

    // 合并统计数据和名称
    const mergedStats = useMemo(() => {
        if (!apiKeyStats) return [];
        return apiKeyStats
            .map((stat) => ({
                api_key_id: stat.api_key_id,
                name: nameMap.get(stat.api_key_id)?.name ?? `Key #${stat.api_key_id}`,
                enabled: nameMap.get(stat.api_key_id)?.enabled ?? false,
                total_cost_raw: stat.total_cost.raw,
                total_cost: stat.total_cost,
                request_count: stat.request_count,
                total_token: stat.total_token,
                input_cost: stat.input_cost,
                output_cost: stat.output_cost,
            }))
            .sort((a, b) => b.total_cost_raw - a.total_cost_raw);
    }, [apiKeyStats, nameMap]);

    // 计算总费用用于百分比
    const maxCost = useMemo(() => {
        if (mergedStats.length === 0) return 1;
        return mergedStats[0].total_cost_raw || 1;
    }, [mergedStats]);

    if (mergedStats.length === 0) {
        return (
            <div className="rounded-3xl bg-card border-card-border border p-5 text-card-foreground">
                <div className="flex items-center gap-2 mb-3">
                    <KeyRound className="w-4 h-4 text-muted-foreground" />
                    <h3 className="font-semibold text-base">{t('title')}</h3>
                </div>
                <p className="text-sm text-muted-foreground">{t('noData')}</p>
            </div>
        );
    }

    return (
        <div className="rounded-3xl bg-card border-card-border border p-5 text-card-foreground">
            <div className="flex items-center gap-2 mb-4">
                <KeyRound className="w-4 h-4 text-muted-foreground" />
                <h3 className="font-semibold text-base">{t('title')}</h3>
            </div>
            <div className="space-y-3">
                {mergedStats.map((stat, i) => (
                    <motion.div
                        key={stat.api_key_id}
                        className="rounded-2xl border border-border/50 bg-background/40 p-3 flex flex-col gap-2"
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{
                            duration: 0.4,
                            ease: EASING.easeOutExpo,
                            delay: i * 0.05,
                        }}
                    >
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2 min-w-0">
                                <div className={`w-2 h-2 rounded-full shrink-0 ${stat.enabled ? 'bg-green-500' : 'bg-muted-foreground/30'}`} />
                                <span className="text-sm font-medium truncate">{stat.name}</span>
                            </div>
                            <div className="flex items-center gap-3 shrink-0 text-xs text-muted-foreground">
                                <span className="flex items-center gap-1">
                                    <MessageSquare className="size-3" />
                                    <AnimatedNumber value={stat.request_count.formatted.value} />
                                    {stat.request_count.formatted.unit}
                                </span>
                                <span className="flex items-center gap-1">
                                    <Zap className="size-3" />
                                    <AnimatedNumber value={stat.total_token.formatted.value} />
                                    {stat.total_token.formatted.unit}
                                </span>
                                <span className="flex items-center gap-1 font-medium text-foreground">
                                    <DollarSign className="size-3 text-emerald-500" />
                                    <AnimatedNumber value={stat.input_cost.formatted.value} />
                                    {stat.input_cost.formatted.unit}
                                </span>
                            </div>
                        </div>
                        <Progress
                            value={Math.min(100, (stat.total_cost_raw / maxCost) * 100)}
                            className="h-1.5 *:data-[slot=progress-indicator]:bg-primary"
                        />
                    </motion.div>
                ))}
            </div>
        </div>
    );
}
