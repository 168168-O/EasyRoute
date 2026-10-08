#!/usr/bin/env python3
"""Run the high-EQ prompt against any OpenAI-compatible endpoint.
Env: BASE_URL (default local ollama), API_KEY, MODEL. Example for DeepSeek:
  BASE_URL=https://api.deepseek.com API_KEY=sk-... MODEL=deepseek-chat python3 run_tests.py
"""
import json, os, re, sys, time, urllib.request
HERE=os.path.dirname(os.path.abspath(__file__))
BASE=os.environ.get('BASE_URL','http://127.0.0.1:11434/v1').rstrip('/')
KEY=os.environ.get('API_KEY','ollama'); MODEL=os.environ.get('MODEL','qwen2.5:7b')
SYS=open(os.path.join(HERE,'system_prompt.txt'),encoding='utf-8').read()
TPL=open(os.path.join(HERE,'user_template.txt'),encoding='utf-8').read()
PERSONA='我待人热心、说话实在，不喜欢客套和油腻的话。跟朋友随意，跟长辈和领导有礼貌。偶尔用个表情，不用网络烂梗。'
TAGS={'温暖体贴','幽默化解','真诚夸赞','委婉拒绝','轻松简短','共情倾听','给台阶','得体坚持','诚恳道歉','积极推进'}
GREASY=['宝子','亲爱的','家人们','绝绝子','yyds','么么哒','小可爱','宝贝','你真优秀','你太棒了','亲~','亲，']
def build(c,n=5):
    return TPL.format(relation=c['relation'],persona=PERSONA,goal=c['goal'] or '无',
        points='\n'.join('- '+p for p in c['points']) or '（无）',styles='自动',n=n,
        history='\n'.join(c['history']) or '（无）',last=c['last'])
def call(user):
    body={'model':MODEL,'temperature':0.8,'top_p':0.95,'max_tokens':900,
          'response_format':{'type':'json_object'},
          'messages':[{'role':'system','content':SYS},{'role':'user','content':user}]}
    req=urllib.request.Request(BASE+'/chat/completions',data=json.dumps(body).encode(),
        headers={'Content-Type':'application/json','Authorization':'Bearer '+KEY})
    t=time.time(); r=json.load(urllib.request.urlopen(req,timeout=600))
    return r['choices'][0]['message']['content'], time.time()-t
def check(d):
    w=[]; rs=d.get('replies',[])
    if not 3<=len(rs)<=5: w.append(f'条数={len(rs)}')
    tags=[r.get('tag') for r in rs]
    if len(set(tags))!=len(tags): w.append('标签重复')
    w+= [f'未知标签:{t}' for t in tags if t not in TAGS]
    for r in rs:
        if len(r.get('text',''))>60: w.append(f'过长({len(r["text"])}字)')
        if len(r.get('why',''))>24: w.append(f'why过长({len(r["why"])}字)')
        w+= [f'油腻词:{g}' for g in GREASY if g in r.get('text','')]
    return w
out=[f'# 测试结果 · {MODEL} · {time.strftime("%Y-%m-%d %H:%M")}\n']
for c in json.load(open(os.path.join(HERE,'cases.json'),encoding='utf-8')):
    for attempt in range(2):  # app behaviour: retry once on bad/empty JSON
        raw,dt=call(build(c))
        try:
            d=json.loads(re.search(r'\{.*\}',raw,re.S).group(0)); warn=check(d)
            if d.get('replies'): break
        except Exception as e: d=None; warn=[f'JSON解析失败:{e}']
    if attempt: warn.append('重试过1次')
    out.append(f'## {c["id"]}  （{dt:.1f}s）\n对方：{c["last"]}  \n我想表达：{c["goal"]}\n')
    if d:
        rd=d.get('read',{}); out.append(f'- 读情绪：{rd.get("emotion")}｜意图：{rd.get("intent")}｜避免：{rd.get("avoid")}')
        for i,r in enumerate(d.get('replies',[])):
            star=' ★' if i==d.get('best') else ''
            out.append(f'- **[{r.get("tag")}]{star}** {r.get("text")}  \n  ↳ {r.get("why")}')
    else: out.append('```\n'+raw+'\n```')
    out.append(f'\n检查：{"通过" if not warn else "；".join(warn)}\n')
    print(c['id'],'done',f'{dt:.1f}s',warn,flush=True)
fn=os.path.join(HERE,f'results_{re.sub(r"[^A-Za-z0-9.]+","_",MODEL)}.md')
open(fn,'w',encoding='utf-8').write('\n'.join(out)); print('saved',fn)
