
import re
import html
import subprocess
import os

def process_html(filename):
    with open(filename, 'r', encoding='utf-8') as f:
        content = f.read()

    # Find <pre class="mermaid"><code>...</code></pre> blocks
    # Note: Pandoc might put attributes differently, but we saw <pre class="mermaid"><code> in the output.
    # We use DOTALL to match newlines.
    pattern = re.compile(r'<pre class="mermaid"><code>(.*?)</code></pre>', re.DOTALL)

    def replacer(match):
        code = match.group(1)
        # Unescape HTML entities (e.g. &gt; -> >)
        code = html.unescape(code)
        return f'<div class="mermaid">{code}</div>'

    new_content = pattern.sub(replacer, content)

    # Inject Mermaid JS script at the end of body
    mermaid_script = """
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
    <script>
        mermaid.initialize({ startOnLoad: true, theme: 'default' });
    </script>
    </body>
    """
    
    if "</body>" in new_content:
        new_content = new_content.replace("</body>", mermaid_script)
    else:
        new_content += mermaid_script

    # Inject some CSS to center the diagrams
    style = """
    <style>
        .mermaid {
            text-align: center;
            margin: 2em 0;
        }
    </style>
    </head>
    """
    if "</head>" in new_content:
        new_content = new_content.replace("</head>", style)

    with open(filename, 'w', encoding='utf-8') as f:
        f.write(new_content)

if __name__ == "__main__":
    process_html("design.html")
    print("Successfully processed design.html with Mermaid JS")
