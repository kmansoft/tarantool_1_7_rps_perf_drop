#!/usr/bin/env tarantool

--[[
We use millisecond timestamps (passed in by caller)
--]]
local TIME_MS_SECOND = 1000ULL
local TIME_MS_10_MIN = TIME_MS_SECOND * 60ULL * 10ULL
local TIME_MS_1_HOUR = TIME_MS_SECOND * 60ULL * 60ULL

--[[
Startup info
--]]
print('Push database starting up')
print('Lua version: ', _VERSION)

--[[
Database config
--]]

local config_bind = "127.0.0.1:60501"

local config_box = {
	logger = 'push_db_server.log',
	log_level = 5,
	snapshot_period = 3600,
	snapshot_count = 3,
	rows_per_wal = 100000,
	listen = config_bind
}

box.cfg(config_box)

print('DB listening on:', config_bind)
print('Tarantool version: ', box.info.version)

local user = box.session.user()
print('Current user:', user)

--[[
"subs" space:
--]]

space_subs = box.space.subs
if not space_subs then
    space_subs = box.schema.space.create('subs')
    space_subs:create_index('primary', { parts = {2, 'STR', 5, 'STR'}, type = 'HASH' })
    space_subs:create_index('dev_id', { parts = {2, 'STR'}, type = 'TREE', unique = false})
    space_subs:create_index('ping_ts', { parts = {3, 'NUM'}, type = 'TREE', unique = false })
end

--[[
"devs" space:
--]]

space_devs = box.space.devs
if not space_devs then
    space_devs = box.schema.space.create('devs')
    space_devs:create_index('primary', { parts = {1, 'STR'}, type = 'HASH' })
    space_devs:create_index('ping_ts', { parts = {5, 'NUM'}, type = 'TREE', unique = false})
    space_devs:create_index('change_ts', { parts = {6, 'NUM'}, type = 'TREE', unique = false })
end

--[[
Access
--]]

print('Access control:', 'guest')
box.once('access-guest', function()
		box.schema.user.grant('guest', 'read,write,execute', 'universe', nil)
	end)

--[[
Stats
--]]

local slab_info = box.slab.info()

print ('Slab used : ', math.floor(slab_info.arena_used / 1024))
print ('Slab size : ', math.floor(slab_info.arena_size / 1024))
print ('Slab quota: ', math.floor(slab_info.quota_size / 1024))
print ('Existing devs count: ', space_devs:len())
print ('Existing subs count: ', space_subs:len())

--[[
Error codes
--]]

local RES_OK = 0

local RES_ERR_UNKNOWN_DEV_ID = -1
local RES_ERR_UNKNOWN_SUB_ID = -2
local RES_ERR_MISMATCHING_SUB_ID_DEV_ID = -3

--[[
Device registration
--]]

function push_CreateDev(dev_id, auth, push_token, push_tech, now)
	local t_dev = {dev_id, auth, push_token, push_tech, now, now - TIME_MS_1_HOUR, 0, 0, 0, 0, 0}
	-- dev.ping_ts: now
	space_devs:upsert(t_dev, {{'=', 5, now}})

	return space_devs:get(dev_id)
end

--[[
Create a sub
--]]

function push_CreateSub(dev_id, folder_id, sub_id, now)
	local res = RES_OK

	--  Update dev.ping_ts: now
	local t_dev = space_devs:update(dev_id, {{'=', 5, now}})

	if t_dev == nil
	then
		res = RES_ERR_UNKNOWN_DEV_ID
	else
		-- Upsert the sub
		t_sub = {sub_id, dev_id, now + TIME_MS_10_MIN, now - TIME_MS_1_HOUR, folder_id, 0, 0}
		space_subs:upsert(t_sub, {
			-- sub_id
			{'=', 1, sub_id},
			-- ping_ts: now + 10 min
			{'=', 3, now + TIME_MS_10_MIN},
			-- change_ts: now - 1 hour
			{'=', 4, now - TIME_MS_1_HOUR},
			-- ews_is_alive: false
			{'=', 6, 0},
			-- ews_is_dead: false
			{'=', 7, 0}})
	end

	return res
end

--[[
Sub ping / change
--]]

function push_PingSub(dev_id, folder_id, sub_id, set_ping_ts)
	local res = RES_OK

	-- Update sub.ping_ts
	local t_sub = space_subs:update({dev_id, folder_id}, {{'=', 3, set_ping_ts}})

	if t_sub == nil
	then
		res = RES_ERR_UNKNOWN_SUB_ID
	elseif t_sub[1] ~= sub_id
	then
		res = RES_ERR_MISMATCHING_SUB_ID_DEV_ID
	elseif t_sub[6] ~= 1 or t_sub[7] ~= 0
	then
		-- Update sub
		space_subs:update({dev_id, folder_id}, {
			-- ews_is_alive
			{'=', 6, 1},
			-- ews_is_dead
			{'=', 7, 0}})
	end

	return res
end

function push_ChangeSub(dev_id, folder_id, sub_id, now, delta, priority)
	local res = RES_OK

	-- Update sub
	local set_change_ts = now + delta
	local t_sub = space_subs:update({dev_id, folder_id}, {
			-- ping_ts
			{'=', 3, set_change_ts},
			-- change_ts
			{'=', 4, set_change_ts}})

	if t_sub == nil
	then
		res = RES_ERR_UNKNOWN_SUB_ID
	elseif t_sub[1] ~= sub_id
	then
		res = RES_ERR_MISMATCHING_SUB_ID_DEV_ID
	else
		-- Update sub
		if t_sub[6] ~= 1 or t_sub[7] ~= 0
		then
			-- Update sub
			space_subs:update({dev_id, folder_id}, {
				-- ews_is_alive
				{'=', 6, 1},
				-- ews_is_dead
				{'=', 7, 0}})
		end

		box.begin()

		-- Update dev
		local t_dev = space_devs:update(dev_id, {
			-- change_ts
			{'=', 6, set_change_ts},
			-- change_count
			{'+', 7, 1},
			-- change_priority
			{'|', 8, priority}})
		if t_dev
		then
			-- Check dev.change_count and adjust dev.change_ts if neeeded
			if t_dev[7] > 1 and t_dev[7] < 5
			then
				set_change_ts = now + t_dev[7] * delta
				space_devs:update(dev_id, {{'=', 6, set_change_ts}})
			end
		end

		box.commit()
	end

	return res
end
