'use client';

import { useEffect, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { useModelList } from '@/api/endpoints/model';
import { useChannelList } from '@/api/endpoints/channel';
import { ModelItem } from './Item';
import { usePaginationStore, useSearchStore } from '@/components/modules/toolbar';
import { EASING } from '@/lib/animations/fluid-transitions';
import { useGridPageSize } from '@/hooks/use-grid-page-size';
import { Tabs, TabsList, TabsTrigger, TabsContents, TabsContent } from '@/components/animate-ui/primitives/animate/tabs';

/** Model item height: h-28 = 112px */
const MODEL_ITEM_HEIGHT = 112;

export function Model() {
    const [activeChannelId, setActiveChannelId] = useState<number | undefined>(undefined);
    const { data: allModels } = useModelList();
    const { data: channelsData } = useChannelList();

    const pageKey = 'model' as const;
    const pageSize = useGridPageSize({
        itemHeight: MODEL_ITEM_HEIGHT,
        gap: 16,
        columns: { default: 1, md: 2, xl: 3 },
    });
    const searchTerm = useSearchStore((s) => s.getSearchTerm(pageKey));
    const page = usePaginationStore((s) => s.getPage(pageKey));
    const setPage = usePaginationStore((s) => s.setPage);
    const setTotalItems = usePaginationStore((s) => s.setTotalItems);
    const setPageSize = usePaginationStore((s) => s.setPageSize);
    const direction = usePaginationStore((s) => s.getDirection(pageKey));

    // 构建渠道列表（每个渠道一个标签页）
    const channels = useMemo(() => {
        if (!channelsData || channelsData.length === 0) {
            return [];
        }

        return channelsData
            .filter(ch => ch.raw.enabled) // 只显示启用的渠道
            .map(ch => ({
                id: ch.raw.id,
                name: ch.raw.name,
                type: ch.raw.type,
            }))
            .sort((a, b) => a.id - b.id);
    }, [channelsData]);

    // 根据选中的渠道筛选模型
    const filteredModels = useMemo(() => {
        if (!allModels) return [];

        let models = [...allModels];

        // 如果选择了特定渠道，只显示该渠道的模型
        if (activeChannelId !== undefined) {
            models = models.filter(m => m.channel_id === activeChannelId);
        }

        // 搜索过滤
        if (searchTerm.trim()) {
            const term = searchTerm.toLowerCase();
            models = models.filter((m) => m.name.toLowerCase().includes(term));
        }

        // 排序
        return models.sort((a, b) => a.name.localeCompare(b.name));
    }, [allModels, activeChannelId, searchTerm]);

    // Sync to store for Toolbar to display pagination info
    useEffect(() => {
        setTotalItems(pageKey, filteredModels.length);
        setPageSize(pageKey, pageSize);
    }, [filteredModels.length, pageSize, pageKey, setTotalItems, setPageSize]);

    // Reset to page 1 when search term changes
    useEffect(() => {
        setPage(pageKey, 1);
    }, [searchTerm, activeChannelId, pageKey, setPage]);

    const pagedModels = useMemo(() => {
        const start = (page - 1) * pageSize;
        return filteredModels.slice(start, start + pageSize);
    }, [filteredModels, page, pageSize]);

    return (
        <div className="space-y-4">
            <Tabs
                value={activeChannelId === undefined ? 'all' : String(activeChannelId)}
                onValueChange={(val) => setActiveChannelId(val === 'all' ? undefined : Number(val))}
            >
                <div className="rounded-2xl border border-border bg-card p-2 custom-shadow">
                    <TabsList className="flex gap-2 relative overflow-x-auto">
                        <TabsTrigger
                            value="all"
                            className="flex-shrink-0 px-4 py-2 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors relative z-10 rounded-xl data-[state=active]:text-foreground"
                        >
                            全部
                        </TabsTrigger>
                        {channels.map((channel) => (
                            <TabsTrigger
                                key={channel.id}
                                value={String(channel.id)}
                                className="flex-shrink-0 px-4 py-2 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors relative z-10 rounded-xl data-[state=active]:text-foreground"
                            >
                                {channel.name}
                            </TabsTrigger>
                        ))}
                    </TabsList>
                </div>

                <TabsContents className="mt-3">
                    <TabsContent value="all">
                        <AnimatePresence mode="popLayout" initial={false} custom={direction}>
                            <motion.div
                                key={`model-page-${page}-all`}
                                custom={direction}
                                variants={{
                                    enter: (d: number) => ({ x: d >= 0 ? 24 : -24, opacity: 0 }),
                                    center: { x: 0, opacity: 1 },
                                    exit: (d: number) => ({ x: d >= 0 ? -24 : 24, opacity: 0 }),
                                }}
                                initial="enter"
                                animate="center"
                                exit="exit"
                                transition={{ duration: 0.25, ease: EASING.easeOutExpo }}
                            >
                                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                                    <AnimatePresence mode="popLayout">
                                        {pagedModels.map((model, index) => (
                                            <motion.div
                                                key={`model-${model.name}-${model.channel_id}`}
                                                initial={{ opacity: 0, y: 20 }}
                                                animate={{ opacity: 1, y: 0 }}
                                                exit={{
                                                    opacity: 0,
                                                    scale: 0.95,
                                                    transition: { duration: 0.2 }
                                                }}
                                                transition={{
                                                    duration: 0.45,
                                                    ease: EASING.easeOutExpo,
                                                    delay: index === 0 ? 0 : Math.min(0.08 * Math.log2(index + 1), 0.4),
                                                }}
                                                layout={!searchTerm.trim()}
                                            >
                                                <ModelItem model={model} />
                                            </motion.div>
                                        ))}
                                    </AnimatePresence>
                                </div>
                            </motion.div>
                        </AnimatePresence>
                    </TabsContent>
                    {channels.map((channel) => (
                        <TabsContent key={channel.id} value={String(channel.id)}>
                            <AnimatePresence mode="popLayout" initial={false} custom={direction}>
                                <motion.div
                                    key={`model-page-${page}-${channel.id}`}
                                    custom={direction}
                                    variants={{
                                        enter: (d: number) => ({ x: d >= 0 ? 24 : -24, opacity: 0 }),
                                        center: { x: 0, opacity: 1 },
                                        exit: (d: number) => ({ x: d >= 0 ? -24 : 24, opacity: 0 }),
                                    }}
                                    initial="enter"
                                    animate="center"
                                    exit="exit"
                                    transition={{ duration: 0.25, ease: EASING.easeOutExpo }}
                                >
                                    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                                        <AnimatePresence mode="popLayout">
                                            {pagedModels.map((model, index) => (
                                                <motion.div
                                                    key={`model-${model.name}-${model.channel_id}`}
                                                    initial={{ opacity: 0, y: 20 }}
                                                    animate={{ opacity: 1, y: 0 }}
                                                    exit={{
                                                        opacity: 0,
                                                        scale: 0.95,
                                                        transition: { duration: 0.2 }
                                                    }}
                                                    transition={{
                                                        duration: 0.45,
                                                        ease: EASING.easeOutExpo,
                                                        delay: index === 0 ? 0 : Math.min(0.08 * Math.log2(index + 1), 0.4),
                                                    }}
                                                    layout={!searchTerm.trim()}
                                                >
                                                    <ModelItem model={model} />
                                                </motion.div>
                                            ))}
                                        </AnimatePresence>
                                    </div>
                                </motion.div>
                            </AnimatePresence>
                        </TabsContent>
                    ))}
                </TabsContents>
            </Tabs>
        </div>
    );
}
