@REM @echo off

del *log
del log\*.log

@REM "C:\Program Files\Microsoft MPI\Bin\mpiexec.exe" -np 100 go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go -m false > run.log 2>&1


for /l %%a in (10, 10, 100) do (
  echo %%a
  mkdir log\%%a-ipfs
  "C:\Program Files\Microsoft MPI\Bin\mpiexec.exe" -np %%a go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go -m false > log\%%a-ipfs\run.log 2>&1
  move log\*.log log\%%a-ipfs
  python log2csv.py log\%%a-ipfs  

    
  mkdir log\%%a-mesh
  "C:\Program Files\Microsoft MPI\Bin\mpiexec.exe" -np %%a go run main.go votenode.go kubo.go datastruct.go crypt.go utils.go -m true > log\%%a-mesh\run.log 2>&1
  move log\*.log log\%%a-mesh
  python log2csv.py log\%%a-mesh  
)