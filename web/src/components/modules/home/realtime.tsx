'use client';

import { useRealtimeStats } from '@/api/endpoints/stats';
import { motion } from 'motion/react';
import { Activity, Gauge, Clock, AlertTriangle, Server } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { EASING } from '@/lib/animations/fluid-transitions';

/**
 * 实时监控仪表盘
 * 显示活跃请求数、QPS、平均响应时间、错误率、在线渠道数
 */
export function RealtimeDashboard() {
    const { data: stats } = useRealtimeStats();
    const t = useTranslations('home.realtime');

    const items = [
        {
            label: t('activeRequests'),
            value: stats?.active_requests ?? 0,
            icon: Activity,
            color: 'bg-blue-500/10 text-blue-500',
            format: (v: number) => String(v),
        },
        {
            label: t('qps'),
            value: stats?.qps ?? 0,
            icon: Gauge,
            color: 'bg-green-500/10 text-green-500',
            format: (v: number) => v.toFixed(1),
        },
        {
            label: t('avgResponseTime'),
            value: stats?.avg_response_ms ?? 0,
            icon: Clock,
            color: 'bg-amber-500/10 text-amber-500',
            format: (v: number) => `${Math.round(v)}ms`,
        },
        {
            label: t('errorRate'),
            value: stats?.error_rate ?? 0,
            icon: AlertTriangle,
            color: (stats?.error_rate ?? 0) > 10
                ? 'bg-destructive/10 text-destructive'
                : 'bg-orange-500/10 text-orange-500',
            format: (v: number) => `${v.toFixed(1)}%`,
        },
        {
            label: t('channelCount'),
            value: stats?.channel_count ?? 0,
            icon: Server,
            color: 'bg-purple-500/10 text-purple-500',
            format: (v: number) => String(v),
        },
    ];

    return (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
            {items.map((item, i) => (
                <motion.section
                    key={item.label}
                    className="rounded-3xl bg-card border-card-border border p-4 text-card-foreground flex flex-col gap-2"
                    initial={{ opacity: 0, y: 20, filter: 'blur(8px)' }}
                    animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
                    transition={{
                        duration: 0.5,
                        ease: EASING.easeOutExpo,
                        delay: i * 0.08,
                    }}
                >
                    <div className="flex items-center gap-2">
                        <div className={`w-8 h-8 rounded-xl flex items-center justify-center shrink-0 ${item.color}`}>
                            <item.icon className="w-4 h-4" />
                        </div>
                        <span className="text-xs text-muted-foreground">{item.label}</span>
                    </div>
                    <div className="text-xl font-semibold tabular-nums">
                        {item.format(item.value)}
                    </div>
                </motion.section>
            ))}
        </div>
    );
}
