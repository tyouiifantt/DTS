# ./log/フォルダにあるファイル名に_agg_が含まれない各種ログファイルを参照し
# amount of sub target: の後に続く数字を抽出し、csvファイルに変換する

import os
import re
import csv
import sys

# フォルダにある各ファイルを参照してファイル名に_agg_が含まれないファイルを抽出
def get_file_list(dir):
    print("dir" + dir)
    file_list = [os.path.join(dir, file) for file in os.listdir(dir)]
    # file_list = [os.path.abspath(path) for path in os.listdir(os.path.abspath(dir))]
    print("file_list")
    print(file_list[0])
    file_list = [file for file in file_list if '_agg_' not in file and '.log' in file]
    return file_list

# ファイルを開き、amount of sub target: の後に続く数字を抽出して配列として返す
def get_amount_list(file):
    with open(file.replace('\\\\', '\\'), 'r') as f:
        lines = f.readlines()
    target_list = "0"
    sig_list = "0"
    target = ':'
    for line in lines:
        if 'amount of sub target: ' in line:
            idx = line.find(target)
            r = line[idx+1:].replace('\n', '')
            print("target" + r)
            target_list = target_list + " , " +r
        elif 'amount of sub Sigs: ' in line:
            idx = line.find(target)
            r = line[idx+1:].replace('\n', '')
            print("sig" +r)
            sig_list = sig_list + " , " +r
    amount_list = [target_list, sig_list]
    return amount_list

# 入力された配列を2行にして、，で区切ってcsvファイルに追記する
def write_csv(dir, amount_list):
    with open(dir +'amount.csv', 'a') as f:
        f.write(amount_list[0] + '\n')
        f.write(amount_list[1] + '\n')
        f.close()



def main(dir):
    files = get_file_list(dir)
    # print(files)
    for file in files:
        print(file)
        amount_list = get_amount_list(file)
        print(amount_list)
        write_csv(dir, amount_list)

# main処理
if __name__ == '__main__':
    args = sys.argv
    if 2 <= len(args):
        main(args[1])
    else:
        main("./")