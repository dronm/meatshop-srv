
REM echo -n "myasmasters:0cca68fc7ad141b2b94542281ba33000" | base64
REM bXlhc21hc3RlcnM6MGNjYTY4ZmM3YWQxNDFiMmI5NDU0MjI4MWJhMzMwMDA=
REM -H "Authorization: Basic bXlhc21hc3RlcnM6MGNjYTY4ZmM3YWQxNDFiMmI5NDU0MjI4MWJhMzMwMDA="
curl -X POST http://127.0.0.1:5000/execute -H "Content-Type: application/json" -H "Authorization: Basic bXlhc21hc3RlcnM6MGNjYTY4ZmM3YWQxNDFiMmI5NDU0MjI4MWJhMzMwMDA=" -d @new_order_data.json
curl -X POST http://127.0.0.1:5000/bin-data -H "Content-Type: application/json" -d @print_order_data.json -o print.pdf

curl -X POST http://127.0.0.1:5000/execute -H "Content-Type: application/json" -d @new_sale_data.json
curl -X POST http://127.0.0.1:5000/execute -H "Authorization: Basic c3BldHNvdjo3NGU3ODgxYWI3M2Y0YmQ0OTE2Y2RmNjFjOWViNmU0Nw==" -H "Content-Type: application/json" -d @check_order_close.json


curl http://92.255.164.212:45432/health
curl http://92.255.164.212:45432/status -H "Authorization: Basic bXlhc21hc3RlcnM6MGNjYTY4ZmM3YWQxNDFiMmI5NDU0MjI4MWJhMzMwMDA="
curl http://92.255.164.212:45432/stop -H "Authorization: Basic bXlhc21hc3RlcnM6MGNjYTY4ZmM3YWQxNDFiMmI5NDU0MjI4MWJhMzMwMDA="
curl http://92.255.164.212:45432/start -H "Authorization: Basic bXlhc21hc3RlcnM6MGNjYTY4ZmM3YWQxNDFiMmI5NDU0MjI4MWJhMzMwMDA="

