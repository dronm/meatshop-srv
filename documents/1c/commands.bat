
REM echo -n "spetsov:74e7881ab73f4bd4916cdf61c9eb6e47" | base64
REM c3BldHNvdjo3NGU3ODgxYWI3M2Y0YmQ0OTE2Y2RmNjFjOWViNmU0Nw==
REM -H "Authorization: Basic c3BldHNvdjo3NGU3ODgxYWI3M2Y0YmQ0OTE2Y2RmNjFjOWViNmU0Nw=="
C:\Base_1C\curl\curl -X POST http://127.0.0.1:5000/execute -H "Content-Type: application/json" -H "Authorization: Basic c3BldHNvdjo3NGU3ODgxYWI3M2Y0YmQ0OTE2Y2RmNjFjOWViNmU0Nw==" -d @new_order_data.json
C:\Base_1C\curl\curl -X POST http://127.0.0.1:5000/bin-data -H "Content-Type: application/json" -d @print_order_data.json -o print.pdf

C:\Base_1C\curl\curl -X POST http://127.0.0.1:5000/execute -H "Content-Type: application/json" -d @new_sale_data.json
C:\Base_1C\curl\curl -X POST http://127.0.0.1:5000/execute -H "Authorization: Basic c3BldHNvdjo3NGU3ODgxYWI3M2Y0YmQ0OTE2Y2RmNjFjOWViNmU0Nw==" -H "Content-Type: application/json" -d @check_order_close.json


C:\Base_1C\curl\curl http://127.0.0.1:5000/status -H "Authorization: Basic c3BldHNvdjo3NGU3ODgxYWI3M2Y0YmQ0OTE2Y2RmNjFjOWViNmU0Nw=="
C:\Base_1C\curl\curl http://127.0.0.1:5000/health
curl http://212.113.243.234:5000/health
curl http://212.113.243.234:5000/status -H "Authorization: Basic c3BldHNvdjo3NGU3ODgxYWI3M2Y0YmQ0OTE2Y2RmNjFjOWViNmU0Nw=="

C:\Base_1C\curl\curl -X POST http://127.0.0.1:5000/stop
C:\Base_1C\curl\curl -X POST http://127.0.0.1:5000/start