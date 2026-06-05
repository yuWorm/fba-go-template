insert into sys_menu (id, title, name, path, sort, icon, type, component, perms, status, display, cache, link, remark, parent_id, created_time, updated_time)
values
(66, 'page.menu.scheduler', 'Scheduler', '/scheduler', 2, 'material-symbols:automation', 0, null, null, 1, 1, 1, '', null, null, '2025-06-26 20:29:06', null),
(67, 'page.menu.schedulerManage', 'SchedulerManage', '/scheduler/manage', 1, 'ix:scheduler', 1, '/scheduler/manage/index', null, 1, 1, 1, '', null, 66, '2025-06-26 20:29:06', null),
(68, 'page.menu.schedulerRecord', 'SchedulerRecord', '/scheduler/record', 2, 'ix:scheduler', 1, '/scheduler/record/index', null, 1, 1, 1, '', null, 66, '2025-06-26 20:29:06', null),
(69, '新增', 'AddScheduler', null, 0, null, 2, null, 'sys:task:add', 1, 0, 1, '', null, 67, '2025-06-26 20:29:06', null),
(70, '修改', 'EditScheduler', null, 0, null, 2, null, 'sys:task:edit', 1, 0, 1, '', null, 67, '2025-06-26 20:29:06', null),
(71, '删除', 'DeleteScheduler', null, 0, null, 2, null, 'sys:task:del', 1, 0, 1, '', null, 67, '2025-06-26 20:29:06', null),
(72, '执行', 'ExecScheduler', null, 0, null, 2, null, 'sys:task:exec', 1, 0, 1, '', null, 67, '2025-06-26 20:29:06', null),
(73, '撤销', 'RevokeTask', null, 0, null, 2, null, 'sys:task:revoke', 1, 0, 1, '', null, 68, '2025-06-26 20:29:06', null);

insert into task_scheduler (id, name, task, args, kwargs, queue, exchange, routing_key, start_time, expire_time, expire_seconds, type, interval_every, interval_period, crontab, one_off, enabled, total_run_count, last_run_time, remark, created_time, updated_time)
values (1, 'Fixture', 'task_demo', null, null, null, null, null, null, null, null, 0, 10, 'seconds', '* * * * *', false, true, 0, null, null, '2025-06-26 20:29:06', null);

insert into task_result (id, task_id, status, result, date_done, traceback, name, args, kwargs, worker, retries, queue)
values (1, 'task-1', 'active', null, '2025-06-26 20:29:06', null, 'task_demo', '[]', '{}', 'worker-1', 0, 'default');
