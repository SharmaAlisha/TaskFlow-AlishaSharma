-- Seed data for TaskFlow
-- Test user: test@example.com / password123

INSERT INTO users (id, name, email, password, created_at) VALUES (
    '550e8400-e29b-41d4-a716-446655440001',
    'Test User',
    'test@example.com',
    '$2a$12$i9QkI2IocYXWkAph.nJFAuyIKvUv7F5IHPmFAzWbV/eCWzcg0O2Fm',
    NOW()
) ON CONFLICT (email) DO NOTHING;

INSERT INTO projects (id, name, description, owner_id, created_at) VALUES (
    '550e8400-e29b-41d4-a716-446655440010',
    'Website Redesign',
    'Q2 redesign of the company website',
    '550e8400-e29b-41d4-a716-446655440001',
    NOW()
) ON CONFLICT (id) DO NOTHING;

INSERT INTO tasks (id, title, description, status, priority, project_id, creator_id, assignee_id, due_date, created_at, updated_at) VALUES
    (
        '550e8400-e29b-41d4-a716-446655440020',
        'Design homepage mockup',
        'Create Figma designs for the new homepage layout',
        'todo',
        'low',
        '550e8400-e29b-41d4-a716-446655440010',
        '550e8400-e29b-41d4-a716-446655440001',
        '550e8400-e29b-41d4-a716-446655440001',
        '2026-05-01',
        NOW(),
        NOW()
    ),
    (
        '550e8400-e29b-41d4-a716-446655440021',
        'Implement responsive navigation',
        'Mobile-first navigation bar with hamburger menu',
        'in_progress',
        'high',
        '550e8400-e29b-41d4-a716-446655440010',
        '550e8400-e29b-41d4-a716-446655440001',
        '550e8400-e29b-41d4-a716-446655440001',
        '2026-04-20',
        NOW(),
        NOW()
    ),
    (
        '550e8400-e29b-41d4-a716-446655440022',
        'Set up CI/CD pipeline',
        'GitHub Actions workflow for automated testing and deployment',
        'done',
        'medium',
        '550e8400-e29b-41d4-a716-446655440010',
        '550e8400-e29b-41d4-a716-446655440001',
        NULL,
        NULL,
        NOW(),
        NOW()
    )
ON CONFLICT (id) DO NOTHING;
