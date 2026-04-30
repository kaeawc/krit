package dev.krit.javafacts;

import com.sun.source.tree.ClassTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.MemberSelectTree;
import com.sun.source.tree.MethodInvocationTree;
import com.sun.source.tree.Tree;
import com.sun.source.util.JavacTask;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import java.io.IOException;
import java.io.PrintWriter;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import javax.lang.model.element.Element;
import javax.lang.model.element.TypeElement;
import javax.lang.model.type.TypeMirror;
import javax.tools.DiagnosticCollector;
import javax.tools.JavaCompiler;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Main {
  private Main() {}

  public static void main(String[] args) throws Exception {
    Options options = Options.parse(args);
    JavaCompiler compiler = ToolProvider.getSystemJavaCompiler();
    if (compiler == null) {
      throw new IllegalStateException("javac is unavailable; run with a JDK, not a JRE");
    }
    DiagnosticCollector<JavaFileObject> diagnostics = new DiagnosticCollector<>();
    try (StandardJavaFileManager files =
        compiler.getStandardFileManager(diagnostics, Locale.ROOT, StandardCharsets.UTF_8)) {
      Iterable<? extends JavaFileObject> units =
          files.getJavaFileObjectsFromStrings(options.files);
      List<String> javacOptions = new ArrayList<>();
      javacOptions.add("-proc:none");
      if (!options.classpath.isEmpty()) {
        javacOptions.add("-classpath");
        javacOptions.add(options.classpath);
      }
      JavacTask task =
          (JavacTask)
              compiler.getTask(null, files, diagnostics, javacOptions, null, units);
      Iterable<? extends CompilationUnitTree> parsed = task.parse();
      task.analyze();
      Trees trees = Trees.instance(task);
      Facts facts = new Facts();
      for (CompilationUnitTree unit : parsed) {
        new Scanner(trees, unit, facts).scan(unit, null);
      }
      writeJson(options.output, facts);
    }
  }

  private static final class Options {
    final String output;
    final String classpath;
    final List<String> files;

    Options(String output, String classpath, List<String> files) {
      this.output = output;
      this.classpath = classpath;
      this.files = files;
    }

    static Options parse(String[] args) {
      String output = "";
      String classpath = "";
      List<String> files = new ArrayList<>();
      for (int i = 0; i < args.length; i++) {
        switch (args[i]) {
          case "--out":
            output = args[++i];
            break;
          case "--classpath":
            classpath = args[++i];
            break;
          default:
            files.add(args[i]);
        }
      }
      if (output.isEmpty()) {
        throw new IllegalArgumentException("--out is required");
      }
      if (files.isEmpty()) {
        throw new IllegalArgumentException("at least one Java source file is required");
      }
      return new Options(output, classpath, files);
    }
  }

  private static final class Facts {
    final List<CallFact> calls = new ArrayList<>();
    final List<ClassFact> classes = new ArrayList<>();
  }

  private static final class CallFact {
    String file;
    long line;
    long col;
    String callee;
    String receiverType;
    String element;
    String returnType;
  }

  private static final class ClassFact {
    String file;
    long line;
    long col;
    String name;
    String qualifiedName;
    final List<String> supertypes = new ArrayList<>();
  }

  private static final class Scanner extends TreePathScanner<Void, Void> {
    private final Trees trees;
    private final CompilationUnitTree unit;
    private final Facts facts;

    Scanner(Trees trees, CompilationUnitTree unit, Facts facts) {
      this.trees = trees;
      this.unit = unit;
      this.facts = facts;
    }

    @Override
    public Void visitMethodInvocation(MethodInvocationTree node, Void unused) {
      TreePath path = getCurrentPath();
      CallFact fact = new CallFact();
      fact.file = unit.getSourceFile().toUri().getPath();
      long start = trees.getSourcePositions().getStartPosition(unit, node);
      fact.line = unit.getLineMap().getLineNumber(start);
      fact.col = unit.getLineMap().getColumnNumber(start);
      fact.callee = calleeName(node);
      Element element = trees.getElement(path);
      fact.element = element == null ? "" : element.toString();
      TypeMirror returnType = trees.getTypeMirror(path);
      fact.returnType = returnType == null ? "" : returnType.toString();
      Tree receiver = receiverExpression(node);
      if (receiver != null) {
        TypeMirror receiverType = trees.getTypeMirror(new TreePath(path, receiver));
        fact.receiverType = receiverType == null ? "" : receiverType.toString();
      } else {
        fact.receiverType = "";
      }
      facts.calls.add(fact);
      return super.visitMethodInvocation(node, unused);
    }

    @Override
    public Void visitClass(ClassTree node, Void unused) {
      TreePath path = getCurrentPath();
      Element element = trees.getElement(path);
      if (element instanceof TypeElement) {
        TypeElement type = (TypeElement) element;
        ClassFact fact = new ClassFact();
        fact.file = unit.getSourceFile().toUri().getPath();
        long start = trees.getSourcePositions().getStartPosition(unit, node);
        fact.line = unit.getLineMap().getLineNumber(start);
        fact.col = unit.getLineMap().getColumnNumber(start);
        fact.name = type.getSimpleName().toString();
        fact.qualifiedName = type.getQualifiedName().toString();
        TypeMirror superclass = type.getSuperclass();
        if (superclass != null) {
          fact.supertypes.add(superclass.toString());
        }
        for (TypeMirror iface : type.getInterfaces()) {
          fact.supertypes.add(iface.toString());
        }
        facts.classes.add(fact);
      }
      return super.visitClass(node, unused);
    }
  }

  private static String calleeName(MethodInvocationTree node) {
    Tree select = node.getMethodSelect();
    if (select instanceof MemberSelectTree) {
      return ((MemberSelectTree) select).getIdentifier().toString();
    }
    return select.toString();
  }

  private static Tree receiverExpression(MethodInvocationTree node) {
    Tree select = node.getMethodSelect();
    if (select instanceof MemberSelectTree) {
      return ((MemberSelectTree) select).getExpression();
    }
    return null;
  }

  private static void writeJson(String output, Facts facts) throws IOException {
    Path path = Path.of(output);
    if (path.getParent() != null) {
      Files.createDirectories(path.getParent());
    }
    try (PrintWriter out = new PrintWriter(Files.newBufferedWriter(path, StandardCharsets.UTF_8))) {
      out.println("{\"version\":1,\"calls\":[");
      for (int i = 0; i < facts.calls.size(); i++) {
        CallFact f = facts.calls.get(i);
        out.printf(
            "  {\"file\":\"%s\",\"line\":%d,\"col\":%d,\"callee\":\"%s\",\"receiverType\":\"%s\",\"element\":\"%s\",\"returnType\":\"%s\"}%s%n",
            json(f.file), f.line, f.col, json(f.callee), json(f.receiverType), json(f.element),
            json(f.returnType), i + 1 == facts.calls.size() ? "" : ",");
      }
      out.println("],\"classes\":[");
      for (int i = 0; i < facts.classes.size(); i++) {
        ClassFact f = facts.classes.get(i);
        out.printf(
            "  {\"file\":\"%s\",\"line\":%d,\"col\":%d,\"name\":\"%s\",\"qualifiedName\":\"%s\",\"supertypes\":[",
            json(f.file), f.line, f.col, json(f.name), json(f.qualifiedName));
        for (int j = 0; j < f.supertypes.size(); j++) {
          out.printf("\"%s\"%s", json(f.supertypes.get(j)), j + 1 == f.supertypes.size() ? "" : ",");
        }
        out.printf("]}%s%n", i + 1 == facts.classes.size() ? "" : ",");
      }
      out.println("]}");
    }
  }

  private static String json(String value) {
    return value == null
        ? ""
        : value.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n").replace("\r", "");
  }
}
