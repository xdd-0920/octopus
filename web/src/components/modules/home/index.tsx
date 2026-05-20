'use client';

import { Activity } from './activity';
import { Total } from './total';
import { StatsChart } from './chart';
import { Rank } from './rank';
import { RealtimeDashboard } from './realtime';
import { APIKeyStatsPanel } from './apikey-stats';
import { PageWrapper } from '@/components/common/PageWrapper';

export function Home() {
    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-6 pb-24 md:pb-4 rounded-t-3xl">
            <RealtimeDashboard />
            <Total />
            <Activity />
            <StatsChart />
            <Rank />
            <APIKeyStatsPanel />
        </PageWrapper>
    );
}
