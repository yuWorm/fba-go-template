insert into sys_dept (id, name, sort, leader, phone, email, status, deleted, parent_id, created_time, updated_time)
values (1, '测试', 0, null, null, null, 1, 0, null, '2025-06-26 20:29:06', null);

insert into sys_menu (id, title, name, path, sort, icon, type, component, perms, status, display, cache, link, remark, parent_id, created_time, updated_time)
values
(1, 'page.dashboard.title', 'Dashboard', '/dashboard', 0, 'ant-design:dashboard-outlined', 0, null, null, 1, 1, 1, '', null, null, '2025-06-26 20:29:06', null),
(2, 'page.dashboard.analytics', 'Analytics', '/analytics', 0, 'lucide:area-chart', 1, '/dashboard/analytics/index', null, 1, 1, 1, '', null, 1, '2025-06-26 20:29:06', null),
(3, 'page.dashboard.workspace', 'Workspace', '/workspace', 1, 'carbon:workspace', 1, '/dashboard/workspace/index', null, 1, 1, 1, '', null, 1, '2025-06-26 20:29:06', null),
(4, 'page.menu.system', 'System', '/system', 1, 'eos-icons:admin', 0, null, null, 1, 1, 1, '', null, null, '2025-06-26 20:29:06', null),
(9, 'page.menu.sysUser', 'SysUser', '/system/user', 2, 'ant-design:user-outlined', 1, '/system/user/index', null, 1, 1, 1, '', null, 4, '2025-06-26 20:29:06', null),
(50, 'page.menu.profile', 'Profile', '/profile', 6, 'ant-design:profile-outlined', 1, '/_core/profile/index', null, 1, 0, 1, '', null, null, '2025-06-26 20:29:06', null);

insert into sys_role (id, name, status, is_filter_scopes, remark, created_time, updated_time)
values (1, '测试', 1, true, null, '2025-06-26 20:29:06', null);

insert into sys_role_menu (id, role_id, menu_id)
values (1, 1, 1), (2, 1, 2), (3, 1, 3), (4, 1, 50);

insert into sys_user (id, uuid, username, nickname, password, email, status, is_superuser, is_staff, is_multi_login, avatar, phone, deleted, join_time, last_login_time, last_password_changed_time, dept_id, created_time, updated_time)
values (1, 'fixture-user', 'admin', '用户88888', '$2b$12$8y2eNucX19VjmZ3tYhBLcOsBwy9w1IjBQE4SSqwMDL5bGQVp2wqS.', 'admin@example.com', 1, true, true, true, null, null, 0, '2025-06-26 20:29:06', '2025-06-26 20:29:06', '2025-06-26 20:29:06', 1, '2025-06-26 20:29:06', null);

insert into sys_user_role (id, user_id, role_id)
values (1, 1, 1);

insert into sys_data_scope (id, name, status, created_time, updated_time)
values (1, '本部门数据权限', 1, '2025-06-26 20:29:06', null);

insert into sys_data_rule (id, name, model, "column", operator, expression, "value", created_time, updated_time)
values (1, '部门 ID 等于当前用户部门', 'Dept', '__dept_id__', 0, 0, '${dept_id}', '2025-06-26 20:29:06', null);

insert into sys_data_scope_rule (id, data_scope_id, data_rule_id)
values (1, 1, 1);

insert into sys_plugin (id, summary, version, description, author, tags, database, depends_on, enabled, built_in)
values
('config', '参数配置', '0.0.2', 'System config plugin', 'wu-clan', '["other"]', '["mysql","postgresql"]', '["admin"]', true, true),
('dict', '数据字典', '0.0.8', 'Dictionary data plugin', 'wu-clan', '["other"]', '["mysql","postgresql"]', '["admin"]', true, true),
('email', '电子邮件', '0.0.3', 'Email captcha plugin', 'wu-clan', '["other"]', '["mysql","postgresql"]', '[]', true, true),
('notice', '通知公告', '0.0.2', 'System notice and announcement plugin', 'wu-clan', '["other"]', '["mysql","postgresql"]', '["admin"]', true, true),
('oauth2', 'OAuth 2.0', '0.0.11', '支持 GitHub、Google 等社交平台登录', 'wu-clan', '["auth"]', '["mysql","postgresql"]', '["admin"]', true, true),
('task', '任务调度', '0.1.0', 'Task scheduler compatibility plugin', 'wu-clan', '["task"]', '["mysql","postgresql"]', '["admin"]', true, true),
('code_generator', '代码生成', '0.1.1', 'Go 版本不实现代码生成插件：AI 时代不再提供动态代码生成业务，仅保留清单标记', 'wu-clan', '["other"]', '["mysql","postgresql"]', '["admin"]', false, true);
