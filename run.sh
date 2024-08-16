rm ./log/*log*

mpirun -np 5 -hostfile hostfile go run main.go > ./log/run.log 2>&1

# go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go -a "http://127.0.0.1:8545"
# go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go -a "http://127.0.0.1:8545"

# mpirun ./runbesu1.sh : ./runbesu2.sh > ./log/run.log 2>&1

# Start a new tmux session named "mysession"
# tmux new-session -d -s mysession

# Split the window horizontally
# tmux split-window -h
# tmux split-window -v
# tmux split-window -v
# tmux split-window -h

# Run first shell script in the first pane
# tmux send-keys -t mysession:0.0 'bash runbesu1.sh > ./log/run1.log 2>&1' C-m 

# Run second shell script in the second pane
# tmux send-keys -t mysession:0.1 'bash runbesu.sh > ./log/run2.log 2>&1'  C-m 

# tmux send-keys -t mysession:0.2 'bash runbesu.sh > ./log/run3.log 2>&1'  C-m 
# tmux send-keys -t mysession:0.3 'bash runbesu.sh > ./log/run4.log 2>&1'  C-m 
# tmux send-keys -t mysession:0.4 'bash runbesu.sh > ./log/run5.log 2>&1'  C-m 

# for i in {1..5}
# do 
#   tmux send-keys -t mysession:0.1 'bash runbesu.sh > ./log/run${i}.log 2>&1'  C-m 
# done

# Attach to the tmux session to view the panes
# tmux attach-session -t mysession



# run(){
#   for test in 1-5; do 
#   #   if test==0;
# #       tmux new-session -d -s ./runbesu1.sh "$1"
# #     else if test==1;
# #       tmux new-session -d -s ./runbesu2.sh "$1"
# #     else ;

# }


# run
#-np 3 -hostfile hostfile go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go > ./log/run.log 2>&1

# # 第1引数の文だけを実行する
# for i in {1..2}
# do
#     j=$(($i*10))
#     echo $j
#     mkdir log/${j}-ipfs
#     mpirun -np ${j} -hostfile hostfile go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go -m false > log/${j}-ipfs/run.log 2>&1
#     mv log/*.log log/${j}-ipfs
#     killall -KILL mainß
#     python log2csv.py log/${j}-ipfs
#     # go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go  -n $i &
# done

# go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go 