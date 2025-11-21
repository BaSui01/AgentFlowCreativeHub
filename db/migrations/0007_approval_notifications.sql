-- 0007_approval_notifications.sql
-- 审批通知与 WebSocket 可靠性增强

-- 1. tenant_configs 扩展 approval_settings
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_name = 'tenant_configs' AND column_name = 'approval_settings'
	) THEN
		ALTER TABLE tenant_configs
			ADD COLUMN approval_settings JSONB DEFAULT '{}'::JSONB;
	END IF;
END $$;

-- 2. approval_requests 扩展通知字段
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'approval_requests') THEN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'approval_requests' AND column_name = 'notify_targets'
		) THEN
			ALTER TABLE approval_requests ADD COLUMN notify_targets JSONB DEFAULT '{}'::JSONB;
		END IF;
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'approval_requests' AND column_name = 'notification_attempts'
		) THEN
			ALTER TABLE approval_requests ADD COLUMN notification_attempts INT NOT NULL DEFAULT 0;
		END IF;
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'approval_requests' AND column_name = 'last_notified_at'
		) THEN
			ALTER TABLE approval_requests ADD COLUMN last_notified_at TIMESTAMPTZ;
		END IF;
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'approval_requests' AND column_name = 'last_notification_error'
		) THEN
			ALTER TABLE approval_requests ADD COLUMN last_notification_error TEXT;
		END IF;
	END IF;
END $$;

-- 3. automation_logs 扩展步骤字段，便于追踪
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'automation_logs') THEN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'automation_logs' AND column_name = 'step_id'
		) THEN
			ALTER TABLE automation_logs ADD COLUMN step_id VARCHAR(100);
		END IF;
	END IF;
END $$;
