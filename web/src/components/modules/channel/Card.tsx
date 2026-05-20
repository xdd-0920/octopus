import {
    MorphingDialog,
    MorphingDialogTrigger,
    MorphingDialogContainer,
    MorphingDialogContent,
} from '@/components/ui/morphing-dialog';
import { CheckCircle2, DollarSign, Key, Layers, MessageSquare, XCircle, PlugZap, Loader2, ChevronDown } from 'lucide-react';
import { type StatsMetricsFormatted } from '@/api/endpoints/stats';
import { type Channel, useEnableChannel, useTestChannel } from '@/api/endpoints/channel';
import { CardContent } from './CardContent';
import { useTranslations } from 'next-intl';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/animate-ui/components/animate/tooltip';
import { Switch } from '@/components/ui/switch';
import { toast } from '@/components/common/Toast';
import { useState, useRef, useEffect } from 'react';

export function Card({ channel, stats, layout = 'grid' }: { channel: Channel; stats: StatsMetricsFormatted; layout?: 'grid' | 'list' }) {
    const t = useTranslations('channel.card');
    const tForm = useTranslations('channel.form');
    const tSections = useTranslations('channel.detail.sections');
    const tMetrics = useTranslations('channel.detail.metrics');
    const enableChannel = useEnableChannel();
    const testChannel = useTestChannel();
    const isListLayout = layout === 'list';
    const [selectedModel, setSelectedModel] = useState<string>('');
    const [showModelDropdown, setShowModelDropdown] = useState(false);
    const dropdownRef = useRef<HTMLDivElement>(null);

    const splitModels = (models: string) =>
        models
            .split(',')
            .map((item) => item.trim())
            .filter(Boolean);

    const allModels = [...new Set([
        ...splitModels(channel.model),
        ...splitModels(channel.custom_model),
    ])];

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
                setShowModelDropdown(false);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    const handleTest = (e: React.MouseEvent) => {
        e.stopPropagation();
        testChannel.mutate(
            { channel_id: channel.id, model: selectedModel || undefined },
            {
                onSuccess: (result) => {
                    if (result.success) {
                        toast.success(`测试通过 (${result.response_time_ms}ms)`);
                    } else {
                        toast.error(result.error || '测试失败');
                    }
                },
                onError: (error) => {
                    toast.error(error.message);
                },
            }
        );
    };

    const modelCount = new Set([
        ...splitModels(channel.model),
        ...splitModels(channel.custom_model),
    ]).size;
    const enabledKeyCount = channel.keys.filter((item) => item.enabled).length;

    const handleEnableChange = (checked: boolean) => {
        enableChannel.mutate(
            { id: channel.id, enabled: checked },
            {
                onSuccess: () => {
                    toast.success(checked ? t('toast.enabled') : t('toast.disabled'));
                },
                onError: (error) => {
                    toast.error(error.message);
                },
            }
        );
    };

    return (
        <MorphingDialog>
            <MorphingDialogTrigger className="w-full">
                <article className="flex flex-col gap-4 rounded-3xl border border-border bg-card text-card-foreground p-4 transition-all duration-300">
                    <header className="relative flex items-center justify-between gap-2">
                        <Tooltip side="top" sideOffset={10} align="center">
                            <TooltipTrigger asChild>
                                <h3 className="text-lg font-bold truncate min-w-0">{channel.name}</h3>
                            </TooltipTrigger>
                            <TooltipContent key={channel.name}>{channel.name}</TooltipContent>
                        </Tooltip>
                        <div className="flex items-center gap-1 shrink-0">
                            <div className="relative" ref={dropdownRef}>
                                <button
                                    className="rounded-xl p-1.5 hover:bg-muted transition-colors disabled:opacity-50 flex items-center gap-1"
                                    disabled={testChannel.isPending || !channel.enabled}
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        setShowModelDropdown(!showModelDropdown);
                                    }}
                                    title={isListLayout ? '测试' : undefined}
                                >
                                    {testChannel.isPending ? (
                                        <Loader2 className="size-4 animate-spin text-muted-foreground" />
                                    ) : (
                                        <PlugZap className="size-4 text-muted-foreground hover:text-foreground" />
                                    )}
                                    {allModels.length > 0 && (
                                        <ChevronDown className="size-3 text-muted-foreground" />
                                    )}
                                </button>
                                {showModelDropdown && allModels.length > 0 && (
                                    <div className="absolute right-0 top-full mt-1 z-50 min-w-[150px] rounded-xl border border-border bg-card shadow-lg py-1 max-h-[200px] overflow-y-auto">
                                        <div
                                            className="px-3 py-1.5 text-xs text-muted-foreground hover:bg-accent cursor-pointer"
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                setSelectedModel('');
                                                setShowModelDropdown(false);
                                                handleTest(e);
                                            }}
                                        >
                                            默认模型
                                        </div>
                                        {allModels.map((model) => (
                                            <div
                                                key={model}
                                                className={`px-3 py-1.5 text-xs cursor-pointer hover:bg-accent ${selectedModel === model ? 'bg-accent font-medium' : ''}`}
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    setSelectedModel(model);
                                                    setShowModelDropdown(false);
                                                    handleTest(e);
                                                }}
                                            >
                                                {model}
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>
                            <Switch
                                checked={channel.enabled}
                                onCheckedChange={handleEnableChange}
                                disabled={enableChannel.isPending}
                                onClick={(e) => e.stopPropagation()}
                            />
                        </div>
                    </header>

                    {isListLayout ? (
                        <dl className="grid grid-cols-2 gap-2 lg:grid-cols-6">
                            <div className="rounded-2xl border border-border/70 bg-background/80 p-2">
                                <dt className="mb-1 flex items-center gap-1 text-xs text-muted-foreground">
                                    <MessageSquare className="size-3.5 text-primary" />
                                    {t('requestCount')}
                                </dt>
                                <dd className="text-sm font-semibold">
                                    {stats.request_count.formatted.value}
                                    <span className="ml-1 text-xs text-muted-foreground">{stats.request_count.formatted.unit}</span>
                                </dd>
                            </div>
                            <div className="rounded-2xl border border-border/70 bg-background/80 p-2">
                                <dt className="mb-1 flex items-center gap-1 text-xs text-muted-foreground">
                                    <Layers className="size-3.5 text-primary" />
                                    {tForm('model')}
                                </dt>
                                <dd className="text-sm font-semibold">{modelCount}</dd>
                            </div>
                            <div className="rounded-2xl border border-border/70 bg-background/80 p-2">
                                <dt className="mb-1 flex items-center gap-1 text-xs text-muted-foreground">
                                    <Key className="size-3.5 text-primary" />
                                    {tSections('keys')}
                                </dt>
                                <dd className="text-sm font-semibold">{enabledKeyCount}/{channel.keys.length}</dd>
                            </div>
                            <div className="rounded-2xl border border-border/70 bg-background/80 p-2">
                                <dt className="mb-1 flex items-center gap-1 text-xs text-muted-foreground">
                                    <CheckCircle2 className="size-3.5 text-emerald-500" />
                                    {tMetrics('successRequests')}
                                </dt>
                                <dd className="text-sm font-semibold">{stats.request_success.formatted.value}</dd>
                            </div>
                            <div className="rounded-2xl border border-border/70 bg-background/80 p-2">
                                <dt className="mb-1 flex items-center gap-1 text-xs text-muted-foreground">
                                    <XCircle className="size-3.5 text-destructive" />
                                    {tMetrics('failedRequests')}
                                </dt>
                                <dd className="text-sm font-semibold">{stats.request_failed.formatted.value}</dd>
                            </div>
                            <div className="rounded-2xl border border-border/70 bg-background/80 p-2">
                                <dt className="mb-1 flex items-center gap-1 text-xs text-muted-foreground">
                                    <DollarSign className="size-3.5 text-primary" />
                                    {t('totalCost')}
                                </dt>
                                <dd className="text-sm font-semibold">
                                    {stats.total_cost.formatted.value}
                                    <span className="ml-1 text-xs text-muted-foreground">{stats.total_cost.formatted.unit}</span>
                                </dd>
                            </div>
                        </dl>
                    ) : (
                        <dl className="grid grid-cols-1 gap-3">
                            <div className="flex items-center justify-between rounded-2xl border border-border/70 bg-background/80 p-2">
                                <div className="flex items-center gap-3">
                                    <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                                        <MessageSquare className="h-5 w-5" />
                                    </span>
                                    <dt className="text-sm text-muted-foreground">{t('requestCount')}</dt>
                                </div>
                                <dd className="text-base">
                                    {stats.request_count.formatted.value}
                                    <span className="ml-1 text-xs text-muted-foreground">{stats.request_count.formatted.unit}</span>
                                </dd>
                            </div>

                            <div className="flex items-center justify-between rounded-2xl border border-border/70 bg-background/80 p-2">
                                <div className="flex items-center gap-3">
                                    <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                                        <DollarSign className="h-5 w-5" />
                                    </span>
                                    <dt className="text-sm text-muted-foreground">{t('totalCost')}</dt>
                                </div>
                                <dd className="text-base">
                                    {stats.total_cost.formatted.value}
                                    <span className="ml-1 text-xs text-muted-foreground">{stats.total_cost.formatted.unit}</span>
                                </dd>
                            </div>
                        </dl>
                    )}

                </article>
            </MorphingDialogTrigger>

            <MorphingDialogContainer>
                <MorphingDialogContent className="w-full md:max-w-xl bg-card text-card-foreground px-4 py-2 rounded-3xl max-h-[90vh] overflow-y-auto">
                    <CardContent channel={channel} stats={stats} />
                </MorphingDialogContent>
            </MorphingDialogContainer>
        </MorphingDialog>
    );
}
