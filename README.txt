Ссылка на обсуждение проблемы:

https://groups.google.com/forum/#!topic/tarantool-ru/qa1LJz8eryU

Просадка RPS в версии 1.7 по сравнению с 1.6.

Процедура:

1 - В одном окне запустить

rm -v 000* ; ./push_db_server.lua

2 - В другом запустить

./run_test_db_server.sh subs ping change

"subs" наполняет базу данных "устройствами" и "папками" (space's "devs" и "subs" в базе).

"ping" меняет информацию "одним" образом, а "change" другим, не создавая дополнительных записей.

Обычно просадка видна на первом прогоне, особенно на этапе "subs".

Например, с ~40K до ~12K (Fedora 26, kernel 4.13, NVME SSD ext4, Intel Core i7 7700, 32 GB RAM)

Если просадки не будет видно, можно

	1) либо не перезапуская Lua-базу, сделать ./run_test_db_server.sh ping change (то есть без "subs")

	2) либо сделать всё заново: убить Lua-скрипт, перезапустить оба шага выше, с удалением 000* файлов.

Ранее, в tarantool 1.6 я такой проблемы не наблюдал.

У меня сейчас 1.7.5-275-g64a4b538f - если откатываюсь на 1.6.9.52, проблема исчезает.

Если запускать тест в /tmp (tmpfs), то проблема также исчезает, даже с 1.7.

Также пробовал 1.7 в Ubuntu 17.10, просадка есть, на 1.6 там откатиться не могу.

Структура кода:

- Lua файл запускается напрямую, у нём прописан #!/usr/bin/env tarantool.

В нём Определяются space's, и задаются несколько процедур

- test_db_server.go - соединяется с Lua файлом и запускает вызовы этих процедур в 20 потоков

- push_db_util.go - утилиты ("модель") для вызова процедур и сопутствующие entities и их marshaling.

- push_config.go - конфигурация для подключения к базе.

Прогон с просадкой, 1.7:

2017/11/03 23:02:13 Subs test, c = 20, n = 100000
2017/11/03 23:02:13 Key length: 40
2017/11/03 23:02:13 Completed  10000 requests,  43026.40 rps
2017/11/03 23:02:13 Completed  20000 requests,  42318.33 rps
2017/11/03 23:02:14 Completed  30000 requests,  41643.19 rps
2017/11/03 23:02:14 Completed  40000 requests,  34316.93 rps
2017/11/03 23:02:14 Completed  50000 requests,  35334.62 rps
2017/11/03 23:02:15 Completed  60000 requests,  35614.75 rps
2017/11/03 23:02:15 Completed  70000 requests,  31732.71 rps
2017/11/03 23:02:15 Completed  80000 requests,  31916.43 rps
2017/11/03 23:02:16 Completed  90000 requests,  32124.96 rps
2017/11/03 23:02:16 Completed 100000 requests,  32093.65 rps
2017/11/03 23:02:16 Completed 110000 requests,  37608.12 rps
2017/11/03 23:02:16 Completed 120000 requests,  37100.83 rps
2017/11/03 23:02:17 Completed 130000 requests,  38290.43 rps
2017/11/03 23:02:17 Completed 140000 requests,  38344.14 rps
2017/11/03 23:02:17 Completed 150000 requests,  35442.24 rps
2017/11/03 23:02:18 Completed 160000 requests,  13468.06 rps
2017/11/03 23:02:19 Completed 170000 requests,  13329.05 rps
2017/11/03 23:02:19 Completed 180000 requests,  13686.10 rps
2017/11/03 23:02:20 Completed 190000 requests,  13400.08 rps
2017/11/03 23:02:21 Completed 200000 requests,  12948.92 rps
2017/11/03 23:02:22 Completed 210000 requests,  13174.36 rps
2017/11/03 23:02:22 Completed 220000 requests,  13504.74 rps
2017/11/03 23:02:23 Completed 230000 requests,  13407.58 rps
2017/11/03 23:02:24 Completed 240000 requests,  13550.60 rps
2017/11/03 23:02:25 Completed 250000 requests,  12988.37 rps
2017/11/03 23:02:25 Completed 260000 requests,  13516.15 rps
2017/11/03 23:02:26 Completed 270000 requests,  13119.88 rps
2017/11/03 23:02:27 Completed 280000 requests,  13515.69 rps
2017/11/03 23:02:28 Completed 290000 requests,  13354.53 rps
2017/11/03 23:02:28 Completed 300000 requests,  13061.61 rps
2017/11/03 23:02:28 Elapsed time: 15.446486994s
2017/11/03 23:02:28 Ops per second: 19421.89

Прогон без просадки, 1.6:

2017/11/03 23:03:24 Subs test, c = 20, n = 100000
2017/11/03 23:03:24 Key length: 40
2017/11/03 23:03:24 Completed  10000 requests,  43931.71 rps
2017/11/03 23:03:24 Completed  20000 requests,  42765.54 rps
2017/11/03 23:03:25 Completed  30000 requests,  42685.07 rps
2017/11/03 23:03:25 Completed  40000 requests,  41868.45 rps
2017/11/03 23:03:25 Completed  50000 requests,  41969.55 rps
2017/11/03 23:03:25 Completed  60000 requests,  41066.40 rps
2017/11/03 23:03:26 Completed  70000 requests,  38955.74 rps
2017/11/03 23:03:26 Completed  80000 requests,  41456.43 rps
2017/11/03 23:03:26 Completed  90000 requests,  41430.36 rps
2017/11/03 23:03:26 Completed 100000 requests,  41063.20 rps
2017/11/03 23:03:26 Completed 110000 requests,  69484.45 rps
2017/11/03 23:03:27 Completed 120000 requests,  69762.79 rps
2017/11/03 23:03:27 Completed 130000 requests,  69792.07 rps
2017/11/03 23:03:27 Completed 140000 requests,  70105.20 rps
2017/11/03 23:03:27 Completed 150000 requests,  70636.84 rps
2017/11/03 23:03:27 Completed 160000 requests,  66260.15 rps
2017/11/03 23:03:27 Completed 170000 requests,  68943.09 rps
2017/11/03 23:03:27 Completed 180000 requests,  70387.99 rps
2017/11/03 23:03:28 Completed 190000 requests,  68853.04 rps
2017/11/03 23:03:28 Completed 200000 requests,  69805.52 rps
2017/11/03 23:03:28 Completed 210000 requests,  67980.34 rps
2017/11/03 23:03:28 Completed 220000 requests,  69485.12 rps
2017/11/03 23:03:28 Completed 230000 requests,  69222.95 rps
2017/11/03 23:03:28 Completed 240000 requests,  68847.53 rps
2017/11/03 23:03:29 Completed 250000 requests,  67450.25 rps
2017/11/03 23:03:29 Completed 260000 requests,  68076.01 rps
2017/11/03 23:03:29 Completed 270000 requests,  68953.27 rps
2017/11/03 23:03:29 Completed 280000 requests,  68938.44 rps
2017/11/03 23:03:29 Completed 290000 requests,  68703.64 rps
2017/11/03 23:03:29 Completed 300000 requests,  62246.05 rps
2017/11/03 23:03:29 Elapsed time: 5.322828519s
2017/11/03 23:03:29 Ops per second: 56361.01

Прогон без просадки, 1.7, /tmp:

2017/11/03 23:05:45 Subs test, c = 20, n = 100000
2017/11/03 23:05:45 Key length: 40
2017/11/03 23:05:45 Completed  10000 requests,  46512.43 rps
2017/11/03 23:05:45 Completed  20000 requests,  45733.46 rps
2017/11/03 23:05:45 Completed  30000 requests,  44026.94 rps
2017/11/03 23:05:46 Completed  40000 requests,  44868.37 rps
2017/11/03 23:05:46 Completed  50000 requests,  44184.96 rps
2017/11/03 23:05:46 Completed  60000 requests,  43865.63 rps
2017/11/03 23:05:46 Completed  70000 requests,  44158.25 rps
2017/11/03 23:05:46 Completed  80000 requests,  43794.64 rps
2017/11/03 23:05:47 Completed  90000 requests,  43884.80 rps
2017/11/03 23:05:47 Completed 100000 requests,  43599.68 rps
2017/11/03 23:05:47 Completed 110000 requests,  73900.38 rps
2017/11/03 23:05:47 Completed 120000 requests,  70671.67 rps
2017/11/03 23:05:47 Completed 130000 requests,  69452.44 rps
2017/11/03 23:05:47 Completed 140000 requests,  67706.47 rps
2017/11/03 23:05:48 Completed 150000 requests,  71957.58 rps
2017/11/03 23:05:48 Completed 160000 requests,  74238.56 rps
2017/11/03 23:05:48 Completed 170000 requests,  72851.73 rps
2017/11/03 23:05:48 Completed 180000 requests,  72912.40 rps
2017/11/03 23:05:48 Completed 190000 requests,  68278.57 rps
2017/11/03 23:05:48 Completed 200000 requests,  66832.60 rps
2017/11/03 23:05:48 Completed 210000 requests,  69100.56 rps
2017/11/03 23:05:49 Completed 220000 requests,  72086.83 rps
2017/11/03 23:05:49 Completed 230000 requests,  70105.91 rps
2017/11/03 23:05:49 Completed 240000 requests,  72751.69 rps
2017/11/03 23:05:49 Completed 250000 requests,  70077.47 rps
2017/11/03 23:05:49 Completed 260000 requests,  72795.16 rps
2017/11/03 23:05:49 Completed 270000 requests,  72269.20 rps
2017/11/03 23:05:49 Completed 280000 requests,  71171.47 rps
2017/11/03 23:05:50 Completed 290000 requests,  70517.26 rps
2017/11/03 23:05:50 Completed 300000 requests,  68638.30 rps
2017/11/03 23:05:50 Elapsed time: 5.081451779s
2017/11/03 23:05:50 Ops per second: 59038.25
