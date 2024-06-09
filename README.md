# IF3230-TuBes-RaftingPengalengan
> Tugas Besar IF3230 Sistem Paralel dan Terdistribusi menugaskan mahasiswa untuk membuat sebuah sistem terdistribusi yang mampu melakukan rafting consensus pada suatu cluster. Sistem ini akan menerima request dari client, kemudian melakukan consensus pada request tersebut. Sistem ini akan terdiri dari beberapa node yang saling berkomunikasi satu sama lain. Sistem ini akan menggunakan algoritma Raft untuk melakukan consensus.

## How to Run Server
1. Use `go run server.go <ip> <port>` to initialize server as cluster leader
2. Use `go run server.go <ip> <port> <leader-ip> <leader-port>` to initialize server as cluster follower

## How to Run Client
1. Use `go run client.go <ip> <port>` to send request to server
2. The ip and port can be any of the server's ip and port that is currently running

## How to Run Request
1. The command can be `ping`, `strlen`, `get`, `set`, `delete`, `append`, or `request log`
2. `ping` will return `PONG` if server is alive
3. `strlen` will return the length of the value of the key
4. `get` will return the value of the key
5. `set` will set the value of the key
6. `delete` will delete the key and return the value of the key
7. `append` will append the value of the key
8. `request log` will return the log of the server

## Anggota Kelompok
- 10023634 - Yudi Kurniawan
- 13520130 - Nelsen Putra
- 13521136 - Ammar Rasyad Chaeroel
- 13521144 - Bintang Dwi Marthen
- 13521157 - Hanif Muhammad Zhafran