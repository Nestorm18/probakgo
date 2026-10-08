-- Last accepted TOTP time step, so a code cannot be replayed within its window.
ALTER TABLE users ADD COLUMN totp_last_step INTEGER NOT NULL DEFAULT 0;
