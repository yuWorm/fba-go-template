set @scheduler_menu_id = (select id from sys_menu where name = 'Scheduler');
set @scheduler_manage_id = (select id from sys_menu where name = 'SchedulerManage');
set @scheduler_record_id = (select id from sys_menu where name = 'SchedulerRecord');

insert into sys_menu (title, name, path, sort, icon, type, component, perms, status, display, cache, link, remark, parent_id, created_time, updated_time)
values
('新增', 'AddScheduler', null, 0, null, 2, null, 'sys:task:add', 1, 0, 1, '', null, @scheduler_manage_id, now(), null),
('修改', 'EditScheduler', null, 0, null, 2, null, 'sys:task:edit', 1, 0, 1, '', null, @scheduler_manage_id, now(), null),
('删除', 'DeleteScheduler', null, 0, null, 2, null, 'sys:task:del', 1, 0, 1, '', null, @scheduler_manage_id, now(), null),
('执行', 'ExecScheduler', null, 0, null, 2, null, 'sys:task:exec', 1, 0, 1, '', null, @scheduler_manage_id, now(), null),
('撤销', 'RevokeTask', null, 0, null, 2, null, 'sys:task:revoke', 1, 0, 1, '', null, @scheduler_record_id, now(), null);

insert into task_scheduler (id, name, task, args, kwargs, queue, exchange, routing_key, start_time, expire_time, expire_seconds, type, interval_every, interval_period, crontab, one_off, enabled, total_run_count, last_run_time, remark, created_time, updated_time)
values (1, 'Fixture', 'task_demo', null, null, null, null, null, null, null, null, 0, 10, 'seconds', '* * * * *', false, true, 0, null, null, now(), null);

insert into task_result (id, task_id, status, result, date_done, traceback, name, args, kwargs, worker, retries, queue)
values (1, 'task-1', 'active', null, now(), null, 'task_demo', '[]', '{}', 'worker-1', 0, 'default');
