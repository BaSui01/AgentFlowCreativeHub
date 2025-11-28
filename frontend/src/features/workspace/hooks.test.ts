import { renderHook, act } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { WorkspaceAPI } from './api';
import { useCommandStatus } from './hooks';
import type { CommandRequestDTO } from './types';

describe('useCommandStatus', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('轮询直到命令进入终态', async () => {
    vi.useFakeTimers();
    const mock = vi.spyOn(WorkspaceAPI, 'getCommandById');
    const first: CommandRequestDTO = {
      id: 'cmd-1',
      agentId: 'agent',
      status: 'queued',
    };
    const second: CommandRequestDTO = {
      id: 'cmd-1',
      agentId: 'agent',
      status: 'completed',
      resultPreview: 'done',
    };
    mock.mockResolvedValueOnce(first).mockResolvedValueOnce(second);

    const { result } = renderHook(() => useCommandStatus('cmd-1'));

    await act(async () => {
      await Promise.resolve();
    });
    expect(mock).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(2000);
      await Promise.resolve();
    });

    expect(mock).toHaveBeenCalledTimes(2);
    expect(result.current.command?.status).toBe('completed');

    await act(async () => {
      vi.advanceTimersByTime(4000);
      await Promise.resolve();
    });

    expect(mock).toHaveBeenCalledTimes(2);
  });

  it('命令ID 置空时清理状态', async () => {
    const mock = vi.spyOn(WorkspaceAPI, 'getCommandById');
    const payload: CommandRequestDTO = {
      id: 'cmd-2',
      agentId: 'agent',
      status: 'completed',
    };
    mock.mockResolvedValue(payload);

    const { result, rerender } = renderHook(({ id }: { id?: string }) => useCommandStatus(id), {
      initialProps: { id: 'cmd-2' },
    });

    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.command?.id).toBe('cmd-2');

    rerender({ id: undefined });
    expect(result.current.command).toBeUndefined();
  });
});
