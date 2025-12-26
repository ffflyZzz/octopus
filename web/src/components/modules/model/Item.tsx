'use client';

import { memo, useCallback, useEffect, useId, useMemo, useState } from 'react';
import { Pencil, Trash2, ArrowDownToLine, ArrowUpFromLine, X } from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { useTranslations } from 'next-intl';
import { useUpdateModel, useDeleteModel, type LLMInfo } from '@/api/endpoints/model';
import { getModelIcon } from '@/lib/model-icons';
import { toast } from '@/components/common/Toast';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/animate-ui/components/animate/tooltip';
import { ModelDeleteOverlay, ModelEditDialog } from './ItemOverlays';
import { cn } from '@/lib/utils';
import {
    MorphingDialog,
    MorphingDialogContainer,
    MorphingDialogContent,
    MorphingDialogDescription,
    MorphingDialogTitle,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';

interface ModelItemProps {
    model: LLMInfo;
}

type EditValues = {
    input: string;
    output: string;
    cache_read: string;
    cache_write: string;
};

function ModelItemContent({ model }: ModelItemProps) {
    const t = useTranslations('model');
    const { setIsOpen, triggerRef, uniqueId, isOpen } = useMorphingDialog();
    const [confirmDelete, setConfirmDelete] = useState(false);
    const [detachCard, setDetachCard] = useState(false);
    const [freezeMorph, setFreezeMorph] = useState(false);
    const instanceId = useId();
    const deleteLayoutId = `delete-btn-${model.name}-${instanceId}`;
    const [editValues, setEditValues] = useState(() => ({
        input: model.input.toString(),
        output: model.output.toString(),
        cache_read: model.cache_read.toString(),
        cache_write: model.cache_write.toString(),
    }));

    const updateModel = useUpdateModel();
    const deleteModel = useDeleteModel();

    const { Avatar: ModelAvatar, color: brandColor } = useMemo(() => getModelIcon(model.name), [model.name]);

    const resetEditValues = useCallback(() => {
        setEditValues({
            input: model.input.toString(),
            output: model.output.toString(),
            cache_read: model.cache_read.toString(),
            cache_write: model.cache_write.toString(),
        });
    }, [model]);

    useEffect(() => {
        if (isOpen) {
            // 弹窗打开后冻结 morph，避免输入变化触发布局动画
            const id = requestAnimationFrame(() => setFreezeMorph(true));
            return () => cancelAnimationFrame(id);
        }
        // 关闭时重置，保留关门 morph 动画
        setFreezeMorph(false);
        setDetachCard(false);
    }, [isOpen]);

    const handleEditOpen = useCallback(() => {
        setConfirmDelete(false);
        resetEditValues();
        setDetachCard(false);
        setIsOpen(true);
    }, [resetEditValues, setIsOpen]);

    const closeDialog = useCallback(() => {
        setFreezeMorph(false); // 重置以保留关门 morph 动画
        setDetachCard(true);
        setIsOpen(false);
    }, [setIsOpen]);

    const handleCancelEdit = () => closeDialog();

    const handleSaveEdit = () => {
        updateModel.mutate({
            name: model.name,
            channel_id: model.channel_id,
            input: parseFloat(editValues.input) || 0,
            output: parseFloat(editValues.output) || 0,
            cache_read: parseFloat(editValues.cache_read) || 0,
            cache_write: parseFloat(editValues.cache_write) || 0,
        }, {
            onSuccess: () => {
                closeDialog();
                toast.success(t('toast.updated'));
            },
            onError: (error) => {
                toast.error(t('toast.updateFailed'), { description: error.message });
            }
        });
    };

    const handleDeleteClick = () => {
        setConfirmDelete(true);
    };
    const handleCancelDelete = () => setConfirmDelete(false);
    const handleConfirmDelete = () => {
        deleteModel.mutate({ name: model.name, channel_id: model.channel_id }, {
            onSuccess: () => {
                setConfirmDelete(false);
                toast.success(t('toast.deleted'));
            },
            onError: (error) => {
                setConfirmDelete(false);
                toast.error(t('toast.deleteFailed'), { description: error.message });
            }
        });
    };

    return (
        <>
            <motion.article
                layoutId={detachCard || freezeMorph ? undefined : `dialog-${uniqueId}`}
                className={cn(
                    'group relative min-h-28 rounded-3xl border border-border bg-card custom-shadow transition-all duration-300 flex items-center gap-3 p-4',
                    confirmDelete && 'z-50'
                )}
            >
                <ModelAvatar size={52} />

                <div className="flex-1 min-w-0 flex flex-col justify-center gap-2">
                    <Tooltip side="top" sideOffset={10} align="start">
                        <TooltipTrigger className='text-base font-semibold text-card-foreground leading-tight truncate'>
                            {model.name}
                        </TooltipTrigger>
                        <TooltipContent>
                            {model.name}
                        </TooltipContent>
                    </Tooltip>

                    <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                        <ArrowDownToLine className="h-3.5 w-3.5" style={{ color: brandColor }} />
                        {t('card.inputCache')}
                        <span className="tabular-nums">{model.input.toFixed(2)}/{model.cache_read.toFixed(2)}$</span>
                    </p>

                    <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                        <ArrowUpFromLine className="h-3.5 w-3.5" style={{ color: brandColor }} />
                        {t('card.outputCache')}
                        <span className="tabular-nums">{model.output.toFixed(2)}/{model.cache_write.toFixed(2)}$</span>
                    </p>
                </div>

                <div
                    className={cn(
                        'shrink-0 flex flex-col justify-between self-stretch',
                        confirmDelete && 'invisible pointer-events-none'
                    )}
                >
                    <motion.button
                        ref={triggerRef}
                        type="button"
                        onClick={handleEditOpen}
                        disabled={confirmDelete}
                        className="h-9 w-9 flex items-center justify-center rounded-lg bg-muted/60 text-muted-foreground transition-colors hover:bg-muted disabled:opacity-50"
                        title={t('card.edit')}
                        aria-label={t('card.edit')}
                    >
                        <Pencil className="h-4 w-4" />
                    </motion.button>

                    <motion.button
                        layoutId={deleteLayoutId}
                        type="button"
                        onClick={handleDeleteClick}
                        disabled={confirmDelete}
                        className="h-9 w-9 flex items-center justify-center rounded-lg bg-destructive/10 text-destructive transition-colors hover:bg-destructive hover:text-destructive-foreground disabled:opacity-50"
                        title={t('card.delete')}
                    >
                        <Trash2 className="h-4 w-4" />
                    </motion.button>
                </div>

                <AnimatePresence>
                    {confirmDelete && (
                        <ModelDeleteOverlay
                            layoutId={deleteLayoutId}
                            isPending={deleteModel.isPending}
                            onCancel={handleCancelDelete}
                            onConfirm={handleConfirmDelete}
                        />
                    )}
                </AnimatePresence>
            </motion.article>

            <MorphingDialogContainer>
                <MorphingDialogContent
                    disableMorph={freezeMorph}
                    onRequestClose={closeDialog}
                    className="w-full max-w-lg rounded-3xl border border-border bg-card text-card-foreground p-6 custom-shadow max-h-[90vh] overflow-y-auto"
                >
                    <MorphingDialogTitle>
                        <header className="mb-4 flex items-center justify-between">
                            <h2
                                id={`motion-ui-morphing-dialog-title-${uniqueId}`}
                                className="text-base font-semibold text-card-foreground"
                            >
                                {model.name}
                            </h2>
                            <motion.button
                                type="button"
                                onClick={closeDialog}
                                className="relative top-0 right-0"
                                initial={{ opacity: 0, scale: 0.85 }}
                                animate={{ opacity: 1, scale: 1 }}
                                exit={{ opacity: 0, scale: 0.85 }}
                                aria-label="Close dialog"
                            >
                                <X className="h-5 w-5" />
                            </motion.button>
                        </header>
                    </MorphingDialogTitle>
                    <MorphingDialogDescription disableLayoutAnimation>
                        <div id={`motion-ui-morphing-dialog-description-${uniqueId}`}>
                            <ModelEditDialog
                                brandColor={brandColor}
                                editValues={editValues}
                                isPending={updateModel.isPending}
                                onChange={setEditValues}
                                onCancel={handleCancelEdit}
                                onSave={handleSaveEdit}
                            />
                        </div>
                    </MorphingDialogDescription>
                </MorphingDialogContent>
            </MorphingDialogContainer>
        </>
    );
}

export const ModelItem = memo(function ModelItem({ model }: ModelItemProps) {
    return (
        <MorphingDialog>
            <ModelItemContent model={model} />
        </MorphingDialog>
    );
});
