'use client';

import { useState } from 'react';
import { useCreateModel } from '@/api/endpoints/model';
import { useChannelList } from '@/api/endpoints/channel';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel, FieldGroup } from '@/components/ui/field';
import {
    MorphingDialogClose,
    MorphingDialogTitle,
    MorphingDialogDescription,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';
import { useTranslations } from 'next-intl';

export function CreateDialogContent() {
    const { setIsOpen } = useMorphingDialog();
    const t = useTranslations('model.create');
    const createModel = useCreateModel();
    const { data: channelsData } = useChannelList();

    const [formData, setFormData] = useState({
        name: '',
        channel_id: '',
        input: '',
        output: '',
        cache_read: '',
        cache_write: '',
    });

    const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!formData.name.trim() || !formData.channel_id) return;

        createModel.mutate({
            name: formData.name.trim(),
            channel_id: parseInt(formData.channel_id),
            input: parseFloat(formData.input) || 0,
            output: parseFloat(formData.output) || 0,
            cache_read: parseFloat(formData.cache_read) || 0,
            cache_write: parseFloat(formData.cache_write) || 0,
        }, {
            onSuccess: () => {
                setFormData({ name: '', channel_id: '', input: '', output: '', cache_read: '', cache_write: '' });
                setIsOpen(false);
            }
        });
    };

    return (
        <div className="w-screen max-w-full md:max-w-xl">
            <MorphingDialogTitle>
                <header className="mb-5 flex items-center justify-between">
                    <h2 className="text-2xl font-bold text-card-foreground">{t('title')}</h2>
                    <MorphingDialogClose
                        className="relative right-0 top-0"
                        variants={{
                            initial: { opacity: 0, scale: 0.8 },
                            animate: { opacity: 1, scale: 1 },
                            exit: { opacity: 0, scale: 0.8 },
                        }}
                    />
                </header>
            </MorphingDialogTitle>
            <MorphingDialogDescription>
                <form onSubmit={handleSubmit}>
                    <FieldGroup className="gap-4">
                        <Field>
                            <FieldLabel htmlFor="model-name">{t('name')}</FieldLabel>
                            <Input
                                id="model-name"
                                value={formData.name}
                                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                                className="rounded-xl"
                            />
                        </Field>
                        <Field>
                            <FieldLabel htmlFor="model-channel">渠道</FieldLabel>
                            <select
                                id="model-channel"
                                value={formData.channel_id}
                                onChange={(e) => setFormData({ ...formData, channel_id: e.target.value })}
                                className="flex h-11 w-full rounded-xl border border-input bg-background px-3 py-2 text-base ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
                            >
                                <option value="">选择渠道</option>
                                {channelsData?.filter(ch => ch.raw.enabled).map(ch => (
                                    <option key={ch.raw.id} value={ch.raw.id}>
                                        {ch.raw.name}
                                    </option>
                                ))}
                            </select>
                        </Field>
                        <div className="grid grid-cols-2 gap-4">
                            <Field>
                                <FieldLabel htmlFor="model-input">{t('input')}</FieldLabel>
                                <Input
                                    id="model-input"
                                    type="number"
                                    step="any"
                                    value={formData.input}
                                    onChange={(e) => setFormData({ ...formData, input: e.target.value })}
                                    className="rounded-xl"
                                />
                            </Field>
                            <Field>
                                <FieldLabel htmlFor="model-output">{t('output')}</FieldLabel>
                                <Input
                                    id="model-output"
                                    type="number"
                                    step="any"
                                    value={formData.output}
                                    onChange={(e) => setFormData({ ...formData, output: e.target.value })}
                                    className="rounded-xl"
                                />
                            </Field>
                            <Field>
                                <FieldLabel htmlFor="model-cache-read">{t('cacheRead')}</FieldLabel>
                                <Input
                                    id="model-cache-read"
                                    type="number"
                                    step="any"
                                    value={formData.cache_read}
                                    onChange={(e) => setFormData({ ...formData, cache_read: e.target.value })}
                                    className="rounded-xl"
                                />
                            </Field>
                            <Field>
                                <FieldLabel htmlFor="model-cache-write">{t('cacheWrite')}</FieldLabel>
                                <Input
                                    id="model-cache-write"
                                    type="number"
                                    step="any"
                                    value={formData.cache_write}
                                    onChange={(e) => setFormData({ ...formData, cache_write: e.target.value })}
                                    className="rounded-xl"
                                />
                            </Field>
                        </div>
                        <Button
                            type="submit"
                            disabled={createModel.isPending || !formData.name.trim() || !formData.channel_id}
                            className="w-full rounded-xl h-11"
                        >
                            {createModel.isPending ? t('submitting') : t('submit')}
                        </Button>
                    </FieldGroup>
                </form>
            </MorphingDialogDescription>
        </div>
    );
}
