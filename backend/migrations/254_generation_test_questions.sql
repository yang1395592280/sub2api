CREATE TABLE generation_test_questions (
    id BIGSERIAL PRIMARY KEY,
    test_kind VARCHAR(24) NOT NULL CHECK (test_kind IN ('pelican', 'intelligence')),
    name VARCHAR(120) NOT NULL,
    prompt TEXT NOT NULL CHECK (char_length(prompt) BETWEEN 1 AND 4000),
    expected_answer VARCHAR(12) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (test_kind <> 'intelligence' OR expected_answer ~ '^[0-9]{1,12}$')
);
CREATE INDEX idx_generation_test_questions_kind ON generation_test_questions(test_kind, id);

INSERT INTO generation_test_questions(test_kind, name, prompt, expected_answer) VALUES
('pelican', '鹈鹕骑自行车 · 2D 动画', '创建一个HTML，内容是绘制一个鹈鹕骑自行车的2D动画，你不需要任何测试', ''),
('intelligence', '糖果形状与口味', $prompt$在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。已知不同口味的糖果和不同形状的数量统计如下表。最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（手中有圆形苹果味配五角星桃子味，或有圆形桃子味配五角星苹果味都满足）

| | 苹果味 | 桃子味 | 西瓜味 |
|---|---|---|---|
| 圆形 | 7 | 9 | 8 |
| 五角星形 | 6 | 6 | 4 |

请给出最终答案数字。
不要联网。$prompt$, '20');

-- Preserve every existing group's custom prompt as a selectable library item.
INSERT INTO generation_test_questions(test_kind, name, prompt, expected_answer)
SELECT DISTINCT ON (s.test_kind, s.prompt) s.test_kind,
    '历史分组题目 · ' || s.group_id, s.prompt, CASE WHEN s.test_kind='intelligence' THEN '20' ELSE '' END
FROM group_pelican_test_schedules s
WHERE NOT EXISTS (SELECT 1 FROM generation_test_questions q WHERE q.test_kind=s.test_kind AND q.prompt=s.prompt)
ORDER BY s.test_kind, s.prompt, s.group_id;

ALTER TABLE group_pelican_test_schedules
    DROP CONSTRAINT group_pelican_test_schedules_interval_minutes_check,
    ADD CONSTRAINT group_pelican_test_schedules_interval_minutes_check CHECK (interval_minutes BETWEEN 5 AND 1440),
    ADD COLUMN question_ids BIGINT[] NOT NULL DEFAULT '{}',
    ADD CONSTRAINT group_pelican_test_schedules_question_limit CHECK (cardinality(question_ids)<=20);
UPDATE group_pelican_test_schedules s SET question_ids=ARRAY[
    (SELECT q.id FROM generation_test_questions q WHERE q.test_kind=s.test_kind AND q.prompt=s.prompt ORDER BY q.id LIMIT 1)
];

-- Snapshots survive later edits/disable operations. No cascading historical FK.
ALTER TABLE account_pelican_tests
    ADD COLUMN question_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN question_name VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN expected_answer VARCHAR(12) NOT NULL DEFAULT '';
UPDATE account_pelican_tests t SET question_id=q.id, question_name=q.name, expected_answer=q.expected_answer
FROM generation_test_questions q WHERE q.test_kind=t.test_kind AND q.prompt=t.prompt;
UPDATE account_pelican_tests SET expected_answer='20', question_name='历史糖果题'
WHERE test_kind='intelligence' AND question_id=0;

DROP INDEX idx_account_pelican_tests_active;
CREATE UNIQUE INDEX idx_account_pelican_tests_active
    ON account_pelican_tests(account_id, test_kind, question_id)
    WHERE status IN ('queued', 'running');
CREATE INDEX idx_account_pelican_tests_batch ON account_pelican_tests(batch_id, account_id, id);

CREATE TABLE generation_test_preferences (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    test_kind VARCHAR(24) NOT NULL CHECK (test_kind IN ('pelican', 'intelligence')),
    question_ids BIGINT[] NOT NULL CHECK (cardinality(question_ids) BETWEEN 1 AND 20),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, test_kind)
);
